package conf

import (
	"fmt"
	"github.com/astaxie/beego"
	"sync"
)

var cfg *AppConfig
var once sync.Once

// AppConfig is the global config of app
type AppConfig struct {
	OracleContractAddress string `json:"oracleContractAddress"`
	NetworkWS             string `json:"networkWS"`
	PrivateKey            string `json:"privateKey"`
}

// SetAppConfig ...
func SetAppConfig() error {
	oracleContractAddress := beego.AppConfig.String("OracleContractAddress")
	if len(oracleContractAddress) == 0 {
		return fmt.Errorf("oracleContractAddress cannot be empty")
	}
	networkWS := beego.AppConfig.String("NetworkWS")
	if len(networkWS) == 0 {
		return fmt.Errorf("networkWS cannot be empty")
	}
	privateKey := beego.AppConfig.String("PrivateKey")
	if len(privateKey) == 0 {
		return fmt.Errorf("privateKey cannot be empty")
	}
	once.Do(
		func() {
			cfg = &AppConfig{
				OracleContractAddress: oracleContractAddress,
				NetworkWS:             networkWS,
				PrivateKey:            privateKey,
			}
		})

	return nil
}

func GetAppConfig() *AppConfig {
	if cfg == nil {
		SetAppConfig()
	}
	return cfg
}
