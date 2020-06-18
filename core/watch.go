package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego/logs"
	simplejson "github.com/bitly/go-simplejson"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
	"math/big"
	"net/http"
	"six-days/oracle-service/conf"
	"strings"
)

const (
	OracelEventName           = "QueryInfo"
	OracelResponseBytesName   = "responseBytes"
	OracelResponseUint256Name = "responseUint256"
	MaxRetryTimes             = 3
	DefaultGasLimit           = 6000000
)

type EventWatch struct {
	BoundContract *bind.BoundContract
	Client        *ethclient.Client
	Config        *conf.AppConfig
	EventChan     chan types.Log
	OracleABI     abi.ABI
	Subscription  ethereum.Subscription
	TransactOpts  *bind.TransactOpts
}

type OracleQueryInfo struct {
	QueryId      [32]byte
	Requester    common.Address
	Fee          *big.Int
	CallbackAddr common.Address
	CallbackFUN  string
	QueryData    []byte
	Raw          types.Log // Blockchain specific contextual infos
}

type QueryRequest struct {
	URL            string   `json:"url,omitempty"`
	ResponseParams []string `json:"responseParams,omitempty"`
}

func NewEventWatch(conf *conf.AppConfig) (*EventWatch, error) {
	conn, err := ethclient.Dial(conf.NetworkWS)
	if err != nil {
		logs.Error("[Start] ethclient dial failed: ", err.Error())
		return nil, err
	}
	abiBytes, err := ioutil.ReadFile("./contract/Oracle.abi")
	if err != nil || len(abiBytes) == 0 {
		return nil, fmt.Errorf("oracle abi file is not exist")
	}
	contractABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return nil, err
	}
	priKey, err := crypto.HexToECDSA(conf.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("HexToECDSA err: %v", err)
	}
	transactOpts := bind.NewKeyedTransactor(priKey)
	boundContract := bind.NewBoundContract(common.HexToAddress(conf.OracleContractAddress), contractABI, conn, conn, conn)
	eventWatch := &EventWatch{
		Config:        conf,
		Client:        conn,
		OracleABI:     contractABI,
		TransactOpts:  transactOpts,
		BoundContract: boundContract,
	}
	return eventWatch, nil
}

// start monitor oracle contract event
func (e *EventWatch) Start() {
	if err := e.subscribeEvent(); err != nil {
		return
	}
	e.dealEvent()
}

func (e *EventWatch) subscribeEvent() error {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			common.HexToAddress(e.Config.OracleContractAddress),
		},
		Topics: [][]common.Hash{
			{e.OracleABI.Events[OracelEventName].ID()},
		},
	}
	events := make(chan types.Log)
	sub, err := e.Client.SubscribeFilterLogs(context.Background(), query, events)
	if err != nil {
		logs.Error("[SubscribeEvent]fail to subscribe event:", err)
		return err
	}
	e.EventChan = events
	e.Subscription = sub
	return nil
}

func (e *EventWatch) dealEvent() {
	for {
		select {
		case err := <-e.Subscription.Err():
			logs.Error("[dealEvent] Subscription err: ", err)
			e.subscribeEvent()
		case vLog := <-e.EventChan:
			// 处理查询请求并回调
			go e.dealQuery(vLog)
		}
	}
}

func (e *EventWatch) dealQuery(vLog types.Log) error {
	queryInfo := &OracleQueryInfo{}
	err := e.OracleABI.Unpack(queryInfo, OracelEventName, vLog.Data)
	if err != nil {
		return fmt.Errorf("[dealQuery] unpack event log failed:%v", err)
	}
	logs.Debug(fmt.Sprintf("QueryId:      %v", queryInfo.QueryId))
	logs.Debug(fmt.Sprintf("Requester:    %v", queryInfo.Requester.Hex()))
	logs.Debug(fmt.Sprintf("Fee:          %v", queryInfo.Fee))
	logs.Debug(fmt.Sprintf("CallbackAddr: %v", queryInfo.CallbackAddr.Hex()))
	logs.Debug(fmt.Sprintf("CallbackFUN:  %v", queryInfo.CallbackFUN))
	logs.Debug(fmt.Sprintf("QueryData:    %v", string(queryInfo.QueryData)))

	reqData := &QueryRequest{}
	if err = json.Unmarshal(queryInfo.QueryData, reqData); err != nil {
		return fmt.Errorf("[dealQuery] unmarshal query data failed:%v", err)
	}
	// check callback fun
	callbackParamStr := queryInfo.CallbackFUN[strings.Index(queryInfo.CallbackFUN, "(")+1 : len(queryInfo.CallbackFUN)-1]
	callbackParams := strings.Split(callbackParamStr, ",")
	if len(callbackParams) != 3 || callbackParams[0] != "bytes32" {
		logs.Error("[dealQuery] invalid CallbackFUN:", queryInfo.CallbackFUN)
		return fmt.Errorf("[dealQuery] invalid CallbackFUN:%v", queryInfo.CallbackFUN)
	}

	retryTime := 0
	stateCode := 1
RetrySendRquenst:
	queryRes, err := e.sendQueryRequest(reqData, callbackParams[2])
	if err != nil {
		logs.Error("[sendQueryRequest] err:", err)
		retryTime++
		if retryTime <= MaxRetryTimes {
			goto RetrySendRquenst
		} else {
			logs.Error("[sendQueryRequest] exceed retry times")
			queryRes = []byte{}
			stateCode = 0
		}
	}

	retryTime = 0
RetrySendResponse:
	err = e.sendQueryResponse(queryRes, uint64(stateCode), queryInfo, callbackParams[2])
	if err != nil {
		logs.Error("[sendQueryResponse] err:", err)
		retryTime++
		if retryTime <= MaxRetryTimes {
			goto RetrySendResponse
		} else {
			logs.Error("[sendQueryResponse] exceed retry times")
			return err
		}
	}
	return nil
}

// sendQueryRequest 根据客户端指定的查询地址发送请求
func (e *EventWatch) sendQueryRequest(reqData *QueryRequest, resParamType string) (interface{}, error) {
	req, err := http.NewRequest("GET", reqData.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("[sendQueryRequest] NewRequest failed: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[sendQueryRequest] http get request failed: %v", err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[sendQueryRequest] read response data failed: %v", err)
	}
	logs.Trace("[sendQueryRequest] get ", reqData.URL, " response is: ", string(body))
	queryRes, err := ParseResponeData(body, reqData.ResponseParams, resParamType)
	if err != nil {
		return nil, err
	}
	return queryRes, nil
}

// sendQueryResponse 将查询到的结果发送给客户端合约指定方法
func (e *EventWatch) sendQueryResponse(res interface{}, stateCode uint64, queryInfo *OracleQueryInfo, resParamType string) error {
	in := []interface{}{
		queryInfo.QueryId,
		queryInfo.CallbackAddr,
		queryInfo.CallbackFUN,
		stateCode,
		res,
	}
	var responseName string
	switch resParamType {
	case "bytes":
		responseName = OracelResponseBytesName
	case "uint256":
		responseName = OracelResponseUint256Name
	default:
		return fmt.Errorf("[SendQueryResponse] unsupport response data type")
	}
	e.TransactOpts.GasLimit = DefaultGasLimit
	transaction, err := e.BoundContract.Transact(e.TransactOpts, responseName, in...)
	if err != nil {
		return fmt.Errorf("[SendQueryResponse] Transact failed: %v", err)
	}
	logs.Trace("[SendQueryResponse] call back tx:", transaction.Hash().Hex())
	return nil
}

// ParseResponeData 解析链下获取到的数据，提取用户所需要的字段，并转换为对应的数据类型
func ParseResponeData(repData []byte, keys []string, resParamType string) (interface{}, error) {
	resData, err := simplejson.NewJson(repData)
	if err != nil {
		return nil, fmt.Errorf("[ParseResponeData] unmarshal response data failed:%v", err)
	}

	for _, paramName := range keys {
		resData = resData.Get(paramName)
	}
	if resData == nil {
		return nil, fmt.Errorf("[ParseResponeData] response data not exist request key:%v", keys)
	}
	var resValue interface{}
	var coverErr error
	switch resParamType {
	case "uint256":
		resUint64Value, coverErr := resData.Uint64()
		if coverErr == nil {
			resValue = big.NewInt(int64(resUint64Value))
		}
	case "uint64":
		resValue, coverErr = resData.Uint64()
	case "int256":
		resInt256Value, coverErr := resData.Int64()
		if coverErr == nil {
			resValue = big.NewInt(resInt256Value)
		}
	case "int64":
		resValue, coverErr = resData.Int64()
	case "address":
		resAddressValueStr, coverErr := resData.String()
		if coverErr == nil {
			resValue = common.HexToAddress(resAddressValueStr)
		}
	case "string":
		resValue, coverErr = resData.String()
	case "bytes":
		resValue, coverErr = resData.Bytes()
	default:
		return nil, fmt.Errorf("[ParseResponeData] unsupport response data type %s", resParamType)
	}
	if coverErr != nil {
		return nil, fmt.Errorf("[ParseResponeData] response data type %s error:%v", resParamType, err)
	}

	return resValue, nil
}
