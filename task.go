package main

import (
	"fmt"
	"reflect"

	"github.com/gofrs/uuid"
)

type State int

const (
	Ready State = iota
	Pending
	Running
	Finished
	Skiped
	Failed
	Rollback
)

func (s State) String() string {
	switch s {
	case Ready:
		return "Ready"
	case Pending:
		return "Pending"
	case Running:
		return "Running"
	case Finished:
		return "Finished"
	case Skiped:
		return "Skiped"
	case Failed:
		return "Failed"
	case Rollback:
		return "Rollback"
	default:
		return "Unknown"
	}
}

type FlowTask struct {
	ID         string
	Name       string
	Action     string
	OnSuccess  []interface{}
	OnError    []interface{}
	OnRollback []interface{}
	Params     map[string]interface{}
	Foreach    interface{}
	Deps       []string
	State      State
	IsRoot     bool
	IsLeaf     bool
	Meta       map[string][]string
	Response   map[string]interface{}
}

func GenerateUUID() string {
	u4, err := uuid.NewV4()
	if err != nil {
		fmt.Printf("failed to generate UUID: %v", err)
	}
	// fmt.Printf("generated Version 4 UUID %v", u4)

	return u4.String()
}

func GetTaskByName(flow *[]FlowTask, name string) int {
	for i := 0; i < len(*flow); i++ {
		if (*flow)[i].Name == name {
			return i
		}
	}

	return -1
}

func GetTasksByName(flow *[]FlowTask, name string) []int {
	var tasks []int

	for i := 0; i < len(*flow); i++ {
		if (*flow)[i].Name == name {
			tasks = append(tasks, i)
		}
	}

	return tasks
}

func GetTasksByNames(flow *[]FlowTask, names []string) []int {
	var tasks []int

	for i := 0; i < len(*flow); i++ {
		for j := 0; j < len(names); j++ {
			if (*flow)[i].Name == names[j] {
				tasks = append(tasks, i)
			}
		}
	}

	return tasks
}

func GetTaskByID(flow *[]FlowTask, uuid string) int {
	for i := 0; i < len(*flow); i++ {
		if (*flow)[i].ID == uuid {
			return i
		}
	}

	return -1
}

func GetOnXTasksString(input []interface{}) []string {
	var ret []string

	for i := 0; i < len(input); i++ {
		// Value type of NextTasks is string
		if value, ok := input[i].(string); ok {
			ret = append(ret, value)
		}

		// Value type of OnSuccess is map
		if value, ok := input[i].(map[interface{}]interface{}); ok {
			runs := value["on-if"].(map[interface{}]interface{})["runs"]
			for i := 0; i < reflect.ValueOf(runs).Len(); i++ {
				s_name := reflect.ValueOf(runs).Index(i).Elem().String()
				ret = append(ret, s_name)
			}
		}
	}

	return ret
}

func MarkNode(flow *[]FlowTask) int {
	for i := 0; i < len(*flow); i++ {
		onSuccessTasks := GetOnXTasksString((*flow)[i].OnSuccess)
		for j := 0; j < len(onSuccessTasks); j++ {
			idx := GetTaskByName(flow, onSuccessTasks[j])
			if idx != -1 {
				(*flow)[idx].IsRoot = false
			}
		}

		onErrorTasks := GetOnXTasksString((*flow)[i].OnError)
		for j := 0; j < len(onErrorTasks); j++ {
			idx := GetTaskByName(flow, onErrorTasks[j])
			if idx != -1 {
				(*flow)[idx].IsRoot = false
			}
		}

		if (len((*flow)[i].OnSuccess) + len((*flow)[i].OnError)) == 0 {
			(*flow)[i].IsLeaf = true
		}
	}

	return 0
}

func GetRoot(flow *[]FlowTask) []string {
	var root_nodes []string
	for i := 0; i < len(*flow); i++ {
		if (*flow)[i].IsRoot {
			root_nodes = append(root_nodes, (*flow)[i].Name)
		}
	}

	return root_nodes
}

func Contains(baseStr []string, str string) bool {
	for i := 0; i < len(baseStr); i++ {
		if baseStr[i] == str {
			return true
		}
	}

	return false
}

func IsSubArr(baseStr []string, subStr []string) bool {
	for i := 0; i < len(subStr); i++ {
		if Contains(baseStr, subStr[i]) {
			continue
		} else {
			return false
		}
	}

	return true
}

func GetsubNodeTasks(flow *[]FlowTask, taskIdx int) []string {
	var ret []string

	onSuccessTasks := GetOnXTasksString((*flow)[taskIdx].OnSuccess)
	ret = append(ret, onSuccessTasks...)
	onErrorTasks := GetOnXTasksString((*flow)[taskIdx].OnError)
	ret = append(ret, onErrorTasks...)

	return ret
}

func FormatStrArr(strArr []string) []string {
	// Wipe out "" items and deduplicate
	for i := 0; i < len(strArr); i++ {
		for j := i + 1; j < len(strArr); j++ {
			if strArr[i] == strArr[j] {
				strArr[i] = ""
			}
		}
	}

	for i := 0; i < len(strArr); i++ {
		if strArr[i] == "" {
			strArr = append(strArr[:i], strArr[i+1:]...)
		}
	}

	return strArr
}

func Intersection(a []string, b []string) []string {
	var rets []string

	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			if a[i] == b[j] {
				rets = append(rets, a[i])
			}
		}
	}

	return rets
}

func Union(a []string, b []string) []string {
	var rets []string

	rets = append(rets, a...)
	rets = append(rets, b...)
	rets = FormatStrArr(rets)

	return rets
}

func Equals(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			if a[i] == b[j] {
				a = append(a[:i], a[i+1:]...)
				b = append(b[:j], b[j+1:]...)
			}
		}
	}

	if len(a) == 0 && len(b) == 0 {
		return true
	}

	return false
}

func Substract(a []string, b []string) []string {
	rets := make([]string, len(a))

	copy(rets, a)
	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			if a[i] == b[j] {
				rets[i] = ""
			}
		}
	}

	rets = FormatStrArr(rets)

	return rets
}

func IsHeritedNode(flow *[]FlowTask, baseIdx int, subIdx int) bool {
	if baseIdx == subIdx {
		// The same node, not herited.
		return false
	}

	// The sub contain all meta in the base, we think they are herited.
	baseForeach := (*flow)[baseIdx].Meta["foreach"]
	// fmt.Println("baseForeach:", baseForeach)
	subForeach := (*flow)[subIdx].Meta["foreach"]
	// fmt.Println("subForeach:", subForeach)
	baseTxn := Substract((*flow)[baseIdx].Meta["txn"], (*flow)[subIdx].Deps)
	// fmt.Println("baseTxn:", baseTxn)
	subTxn := (*flow)[subIdx].Meta["txn"]
	// fmt.Println("subTxn:", subTxn)

	// fmt.Println(IsSubArr(subForeach, baseForeach))
	// fmt.Println(IsSubArr(subTxn, baseTxn))
	if IsSubArr(subForeach, baseForeach) && IsSubArr(subTxn, baseTxn) {
		return true
	}

	return false
}

func CopyMap(inMap map[string]interface{}) map[string]interface{} {
	outMap := make(map[string]interface{}, len(inMap))
	for k, v := range inMap {
		outMap[k] = DeepCopy(v)
	}

	return outMap
}

func DeepCopy(value interface{}) interface{} {
	if valueMap, ok := value.(map[string]interface{}); ok {
		newMap := make(map[string]interface{})
		for k, v := range valueMap {
			newMap[k] = DeepCopy(v)
		}

		return newMap
	} else if valueSlice, ok := value.([]interface{}); ok {
		newSlice := make([]interface{}, len(valueSlice))
		for k, v := range valueSlice {
			newSlice[k] = DeepCopy(v)
		}

		return newSlice
	}

	return value
}
