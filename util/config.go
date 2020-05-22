package util

import "github.com/kataras/iris/v12"

func GetConfig() map[string]interface{} {
	return iris.YAML("config.YAML").GetOther()
}
