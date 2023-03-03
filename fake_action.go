package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FIXME: return type should be string, how to return instances?
func GetRespField(resp map[string]interface{}, path string) interface{} {
	var ret interface{}

	ret = resp
	strArr := strings.Split(path, ".")
	// fmt.Println("ret:", ret)
	// fmt.Println("strArr:", strArr)

	for i := 0; i < len(strArr); i++ {
		if strArr[i] != "" {
			data := ret.(map[string]interface{})
			ret = data[strArr[i]]
		}
	}

	return ret
}

func ReplaceVars(oldStr string, globalFields map[string]interface{}, parentResultData map[string]interface{}, meta map[string][]string) interface{} {
	leftVarMark := "{{ ."
	rightVarMark := " }}"
	startIdx := strings.Index(oldStr, leftVarMark)
	endIdx := strings.Index(oldStr, rightVarMark)
	if startIdx == -1 || endIdx == -1 {
		// have no {{ .xxx }} field
		return oldStr
	}
	globalFieldKey := oldStr[startIdx+len(leftVarMark) : endIdx]
	if globalFields[globalFieldKey] != nil {
		return globalFields[globalFieldKey]
	} else {
		// Multiple parent_results support
		if strings.Contains(oldStr, "{{ .parent_result.") {
			// {{ .parent_result.data.xxx }}
			leftVarMark = "{{ .parent_result."
			rightVarMark = " }}"
			startIdx = strings.Index(oldStr, leftVarMark)
			endIdx = strings.Index(oldStr, rightVarMark)
			path := oldStr[startIdx+len(leftVarMark) : endIdx]
			return GetRespField(parentResultData, path)
		} else if strings.Contains(oldStr, "{{ .each_one }}") {
			// FIXME: {{ .each_one }} should support nested loop
			fmt.Println("each_one:", meta)
			return meta["foreach"][0]
		}
	}

	return nil
}

func GetForeachSlice(foreach interface{}, idx int) string {
	// Value type of v is slice
	if value, ok := foreach.([]interface{}); ok {
		return value[idx].(string)
	}

	return ""
}

func GetForeachLen(foreach interface{}) int {
	var length int

	// Value type of v is string
	if value, ok := foreach.(string); ok {
		fmt.Println("Value:", value)
		length = 1
	}

	// Value type of v is map
	if value, ok := foreach.(map[interface{}]interface{}); ok {
		length = len(value)
	}

	// Value type of v is slice
	if value, ok := foreach.([]interface{}); ok {
		length = len(value)
	}

	return length
}

func RenderTxn(txn []string, globalFields map[string]interface{}, parentResultData map[string]interface{}, meta map[string][]string) []string {
	for i := 0; i < len(txn); i++ {
		txn[i] = ReplaceVars(txn[i], globalFields, parentResultData, meta).(string)
	}

	return txn
}

func RenderOnX(onX []interface{}, globalFields map[string]interface{}, parentResultData map[string]interface{}, meta map[string][]string) []interface{} {
	var rets []interface{}

	// Deal with on-if branches
	for i := 0; i < len(onX); i++ {
		// Value type of OnSuccess is map
		if value, ok := onX[i].(map[interface{}]interface{}); ok {
			onIf := value["on-if"].(map[interface{}]interface{})
			for value := range onIf {
				/*
					for key, value := range onIf {
						strKey := fmt.Sprintf("%v", key)
						strVal := fmt.Sprintf("%v", value)
						fmt.Println("strKey:", strKey)
						fmt.Println("strVal:", strVal)
				*/
				if value, ok := value.([]interface{}); ok {
					// fmt.Println("strrrrrrr:", value)
					for j := 0; j < len(value); j++ {
						value[j] = ReplaceVars(value[j].(string), globalFields, parentResultData, meta)
						// fmt.Println("value[j]:", value[j])
					}
				}
			}
			// fmt.Println("OnX:", onX[i])
			rets = append(rets, onX[i])
		}

		// Value type is string
		if value, ok := onX[i].(string); ok {
			ret := ReplaceVars(value, globalFields, parentResultData, meta)
			rets = append(rets, ret)
		}
	}

	return rets
}

func RenderForeach(foreach interface{}, globalFields map[string]interface{}, parentResultData map[string]interface{}, meta map[string][]string) interface{} {
	if value, ok := foreach.(string); ok {
		// fmt.Println("foreach:", foreach)
		tmpVal := ReplaceVars(value, globalFields, parentResultData, meta)
		// fmt.Println("tmpVal:", tmpVal)
		foreach = tmpVal
	}

	return foreach
}

func RenderParams(inParams map[string]interface{}, globalFields map[string]interface{}, parentResultData map[string]interface{}, meta map[string][]string) map[string]interface{} {
	outParams := CopyMap(inParams)

	for k, v := range outParams {
		// Value type of v is string
		if value, ok := v.(string); ok {
			outParams[k] = ReplaceVars(value, globalFields, parentResultData, meta)
		}

		// Value type of v is map
		if value, ok := v.(map[interface{}]interface{}); ok {
			for key, val := range value {
				strValue := fmt.Sprintf("%v", val)
				value[key] = ReplaceVars(strValue, globalFields, parentResultData, meta)
			}
		}
	}

	return outParams
}

func BuildInputField() map[string]interface{} {
	inputField := make(map[string]interface{})

	inputField["tenantid"] = "tenantid-xxxxxx"
	inputField["token"] = "token-xxxxxx"
	inputField["workflow_id"] = "workflow_id-xxxxxx"
	inputField["server"] = "server-xxxxxx"
	inputField["volumes"] = []string{"volume-1", "volume-2", "volume-3"}

	return inputField
}

func FakeAction(action string, params map[string]interface{}) map[string]interface{} {
	var resp []byte
	var respJson interface{}
	ret := make(map[string]interface{})

	switch action {
	case "test":
		resp = []byte(`{"test"}`)
	case "nova.create_instances":
		resp = []byte(`{"instances":["instance-1", "instance-2", "instance-3"]}`)
	case "nova.create_volumes":
		resp = []byte(`{"volumes":["volume-1", "volume-2", "volume-3"]}`)
	case "nova.check_create_instance_status":
		resp = []byte(`{"volumes":["volume-1", "volume-2", "volume-3"]}`)
	case "nova.check_create_volumes_status":
		resp = []byte(`{"volumes":["volume-1", "volume-2", "volume-3"]}`)
	case "nova.attach_volumes":
		resp = []byte(`{"volumes":["volume-1", "volume-2", "volume-3"]}`)
	case "nova.check_attach_volumes_status":
		resp = []byte(`{"volumes":["volume-1", "volume-2", "volume-3"]}`)
	default:
		resp = []byte(`{}`)
	}
	json.Unmarshal(resp, &respJson)
	ret["data"] = respJson
	ret["code"] = "200"

	return ret
}
