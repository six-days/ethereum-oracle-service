package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"math/big"
	"strings"
	"testing"
)

func TestParseResponeData(t *testing.T) {
	repInfo := "56"
	output, err := ParseResponeData([]byte(repInfo), []string{}, "uint256")
	if err != nil {
		t.Failed()
	}
	require.Equal(t, output, big.NewInt(56))

	repInfo = `{
	"string": "join",
	"data": {
		"uint256": 100002334423,
		"uint64": 102334423,
		"int256": -100002334423,
		"int64": 102334423,
		"bytes": "testbytes",
		"score": {
			"score": 99,
			"address":"0x4E433Ad197a5bAb17274b26b3BE0B37AFE049ea3"
		}
	}
}`
	testList := map[string]interface{}{
		"string":             "join",
		"data;uint256":       big.NewInt(100002334423),
		"data;uint64":        uint64(102334423),
		"data;int256":        big.NewInt(-100002334423),
		"data;int64":         int64(102334423),
		"data;bytes":         []byte("testbytes"),
		"data;score;address": common.HexToAddress("0x4E433Ad197a5bAb17274b26b3BE0B37AFE049ea3"),
	}
	for input, res := range testList {
		inputArr := strings.Split(input, ";")
		output, err := ParseResponeData([]byte(repInfo), inputArr, inputArr[len(inputArr)-1])
		if err != nil {
			t.Failed()
		}
		require.Equal(t, output, res)
	}
}
