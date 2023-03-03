package main

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type QNode struct {
	TaskID string
	Bases  []string
	Subs   []string
}

func PrintTaskQueue(taskQ []QNode) {
	for i := 0; i < len(taskQ); i++ {
		fmt.Println("TaskID:", taskQ[i].TaskID)
		fmt.Println("Bases:", taskQ[i].Bases)
		fmt.Println("Subs:", taskQ[i].Subs)
	}
}

func PrintTask(task *FlowTask, str string) {
	titleLine := strings.Repeat(str, 60)
	contentStart := strings.Repeat(str, 3)
	fmt.Println(titleLine)
	fmt.Println(contentStart, "ID:", (*task).ID)
	fmt.Println(contentStart, "Name:", (*task).Name)
	fmt.Println(contentStart, "Params:", (*task).Params)
	fmt.Println(contentStart, "Action:", (*task).Action)
	fmt.Println(contentStart, "Meta:", (*task).Meta)
	fmt.Println(contentStart, "Response:", (*task).Response)
	fmt.Println(contentStart, "OnSuccess:", (*task).OnSuccess)
	fmt.Println(contentStart, "OnError:", (*task).OnError)
	fmt.Println(titleLine)
}

func PrintTasks(flow *[]FlowTask) {
	for i := 0; i < len(*flow); i++ {
		PrintTask(&(*flow)[i], "*")
	}
}

func FindIDInQueue(taskQ []QNode, ID string) int {
	for i := 0; i < len(taskQ); i++ {
		if taskQ[i].TaskID == ID {
			return i
		}
	}

	return -1
}

func GetTaskState(flow *[]FlowTask, taskQ []QNode, qIdx int) State {
	ret := Running

	taskIdx := GetTaskByID(flow, taskQ[qIdx].TaskID)
	if (*flow)[taskIdx].State == Skiped {
		return Skiped
	}

	for i := 0; i < len(taskQ[qIdx].Bases); i++ {
		taskIdx := GetTaskByID(flow, taskQ[qIdx].Bases[i])
		if (*flow)[taskIdx].State == Finished {
			continue
		} else if (*flow)[taskIdx].State == Ready {
			ret = Ready
		} else {
			ret = (*flow)[taskIdx].State
		}
	}

	// If deps, make sure all tasks with txn ∩ deps == deps finished/skiped
	for i := 0; i < len(*flow); i++ {
		if len((*flow)[taskIdx].Deps) > 0 && len(Substract((*flow)[taskIdx].Deps, Intersection((*flow)[i].Meta["txn"], (*flow)[taskIdx].Deps))) == 0 {
			if (*flow)[i].State == Finished || (*flow)[i].State == Skiped {
				continue
			} else {
				ret = Ready
			}
		}
	}

	return ret
}

func InitFlow(config Config) *[]FlowTask {
	var flow []FlowTask

	// Init first Start node
	newID := GenerateUUID()
	startNode := FlowTask{
		ID:         newID,
		Name:       "START-" + newID, // To avoid repeat name in each workflow
		Action:     "",
		OnSuccess:  nil, // Points to the all real root tasks.
		OnError:    nil,
		OnRollback: nil,
		Params:     BuildInputField(), // Global param list
		Foreach:    nil,
		State:      Finished,
		IsRoot:     false, // I am fake root, not real
		IsLeaf:     false,
		Meta:       nil,
		Response:   nil,
	}
	flow = append(flow, startNode)

	// generate other task nodes
	for i := 0; i < len(config.Tasks); i++ {
		// fmt.Printf("Task %d is: %v\n", i, config.Tasks[i])
		taskNode := FlowTask{
			ID:         GenerateUUID(),
			Name:       config.Tasks[i].Name,
			Action:     config.Tasks[i].Action,
			OnSuccess:  config.Tasks[i].OnSuccess,
			OnError:    config.Tasks[i].OnError,
			OnRollback: config.Tasks[i].OnRollback,
			Params:     config.Tasks[i].Params,
			Foreach:    nil,
			Deps:       config.Tasks[i].Deps,
			State:      Ready,
			IsRoot:     true,
			IsLeaf:     false,
			Meta:       make(map[string][]string),
			Response:   nil,
		}
		taskNode.Meta["txn"] = []string{config.Tasks[i].Txn}
		if config.Tasks[i].Foreach != "" {
			taskNode.Foreach = config.Tasks[i].Foreach
		}

		flow = append(flow, taskNode)
	}

	// Find all root nodes and add them to the start node
	MarkNode(&flow)
	root_nodes := GetRoot(&flow)
	// fmt.Println(root_nodes)
	for i := 0; i < len(root_nodes); i++ {
		flow[0].OnSuccess = append(flow[0].OnSuccess, root_nodes[i])
	}

	return &flow
}

func BuildTaskQueue(flow *[]FlowTask) []QNode {
	// Put all nodes into a operating queue
	var taskQueue []QNode
	lIdx := 0 // All children of nodes on its left side has been added into queue.
	rIdx := 0 // The total size of the task queue.

	// Push the starting node into queue.
	sNode := QNode{
		TaskID: (*flow)[lIdx].ID,
		Bases:  nil,
		Subs:   nil,
	}
	taskQueue = append(taskQueue, sNode)

	for {
		// If there is no elem between lIdx and rIdx, exit
		rIdx = len(*flow)
		if lIdx == rIdx {
			break
		}

		// fmt.Println("lIdx:", lIdx)
		// fmt.Println("ID:", (*flow)[lIdx])
		// Add all sub-nodes
		subTasks := GetsubNodeTasks(flow, lIdx)
		// fmt.Println("heritedTasks:", heritedTasks)
		for i := 0; i < len(subTasks); i++ {
			tmpIdx := GetTaskByName(flow, subTasks[i])
			// foreach field has not been inited
			(*flow)[tmpIdx].Meta["txn"] = append((*flow)[tmpIdx].Meta["txn"], (*flow)[lIdx].Meta["txn"]...)
			(*flow)[tmpIdx].Meta["txn"] = FormatStrArr((*flow)[tmpIdx].Meta["txn"])
			(*flow)[tmpIdx].Meta["txn"] = Substract((*flow)[tmpIdx].Meta["txn"], (*flow)[tmpIdx].Deps)

			qNode := QNode{
				TaskID: (*flow)[tmpIdx].ID,
				Bases:  []string{(*flow)[lIdx].ID},
				Subs:   nil,
			}
			taskQueue = append(taskQueue, qNode)
			qIdx := FindIDInQueue(taskQueue, (*flow)[lIdx].ID)
			taskQueue[qIdx].Subs = append(taskQueue[qIdx].Subs, (*flow)[lIdx].ID)
		}
		lIdx++
		rIdx += len(subTasks)
	}

	return taskQueue
}

func GetOnXTasks(itemsOnX []interface{}) ([]string, []string) {
	var forkRets []string
	var skipRets []string

	// Deal with on-if branches
	for i := 0; i < len(itemsOnX); i++ {
		// Value type of OnSuccess is map
		if value, ok := itemsOnX[i].(map[interface{}]interface{}); ok {
			onIf := value["on-if"].(map[interface{}]interface{})
			mapString := make(map[string][]string)

			for key, value := range onIf {
				strKey := fmt.Sprintf("%v", key)
				for j := 0; j < reflect.ValueOf(value).Len(); j++ {
					mapString[strKey] = append(mapString[strKey], reflect.ValueOf(value).Index(j).Elem().String())
				}
			}

			// fmt.Println(mapString)

			if mapString["eq"] != nil {
				// fmt.Println("Execute on-if eq:")
				if mapString["eq"][0] == mapString["eq"][1] {
					// fmt.Println("eq[0]:", mapString["eq"][0])
					// fmt.Println("eq[1]:", mapString["eq"][1])
					forkRets = append(forkRets, mapString["runs"]...)
				} else {
					skipRets = append(skipRets, mapString["runs"]...)
				}
			}

			if mapString["ne"] != nil {
				// fmt.Println("Execute on-if ne:")
				if mapString["ne"][0] != mapString["ne"][1] {
					// fmt.Println("ne[0]:", mapString["ne"][0])
					// fmt.Println("ne[1]:", mapString["ne"][1])
					forkRets = append(forkRets, mapString["runs"]...)
				} else {
					skipRets = append(skipRets, mapString["runs"]...)
				}
			}
		}

		// Value type is string
		if value, ok := itemsOnX[i].(string); ok {
			forkRets = append(forkRets, value)
		}
	}

	return forkRets, skipRets
}

func SkipOnXTasks(flow *[]FlowTask, taskIdx int, taskQPtr *[]QNode, qIdx int, onXTasks []string) {
	for i := 0; i < len(onXTasks); i++ {
		tmpIdxs := GetTasksByName(flow, onXTasks[i])
		for j := 0; j < len(tmpIdxs); j++ {
			tmpIdx := tmpIdxs[j]
			// FIXME: Consider out of foreach deps...
			if IsHeritedNode(flow, taskIdx, tmpIdx) && (*flow)[tmpIdx].State == Ready {
				// fmt.Println("Mark task as skiped:", (*flow)[tmpIdx].Name)
				(*flow)[tmpIdx].State = Skiped
			}
		}
	}
}

func CloneOnXTasks(flow *[]FlowTask, taskIdx int, taskQPtr *[]QNode, qIdx int, onXTasks []string) {
	if (*flow)[taskIdx].State != Finished {
		// Do nothing and return
		return
	}

	for i := 0; i < len(onXTasks); i++ {
		subTaskIdx := -1
		tmpIdxs := GetTasksByName(flow, onXTasks[i])
		for j := 0; j < len(tmpIdxs); j++ {
			tmpIdx := tmpIdxs[j]
			// a->action1 && b->action1, b found action1 has been finished(a did it), has to fork a new one (no deps)
			if IsHeritedNode(flow, taskIdx, tmpIdx) && (*flow)[tmpIdx].State == Ready {
				// fmt.Println("Deps:", (*flow)[tmpIdx].Deps)
				// Dependency case, assume only one pair txn-deps
				if len((*flow)[tmpIdx].Deps) == 0 {
					subTaskIdx = tmpIdx
					break
				} else if IsSubArr((*flow)[taskIdx].Meta["txn"], (*flow)[tmpIdx].Deps) {
					subTaskIdx = tmpIdx
					// Add Bases dependency
					baseIdx := FindIDInQueue(*taskQPtr, (*flow)[subTaskIdx].ID)
					(*taskQPtr)[baseIdx].Bases = append((*taskQPtr)[baseIdx].Bases, (*flow)[taskIdx].ID)
					break
				}
			}
		}

		// fmt.Println("subTaskIdx:", subTaskIdx)
		if subTaskIdx == -1 {
			// Create a new one
			// fmt.Println("Create a new one:", onXTasks[i])
			newTask := (*flow)[tmpIdxs[0]]
			newTask.ID = GenerateUUID()
			if (*flow)[taskIdx].State != Finished {
				newTask.State = (*flow)[taskIdx].State
			} else {
				newTask.State = Ready
			}

			newTask.Params = CopyMap((*flow)[taskIdx].Params)
			// Inherit Meta
			metaForeach := make([]string, len((*flow)[taskIdx].Meta["foreach"]))
			copy(metaForeach, (*flow)[taskIdx].Meta["foreach"])
			metaTxn := make([]string, len((*flow)[taskIdx].Meta["txn"]))
			copy(metaTxn, (*flow)[taskIdx].Meta["txn"])
			newMeta := make(map[string][]string)
			newMeta["foreach"] = metaForeach
			newMeta["txn"] = Substract(metaTxn, (*flow)[taskIdx].Deps)
			newTask.Meta = newMeta
			// PrintTask(newTask)

			*flow = append(*flow, newTask)
			newQNode := QNode{
				TaskID: newTask.ID,
				Bases:  []string{(*flow)[taskIdx].ID},
				Subs:   nil,
			}
			// Insert the next one after startIdx
			*taskQPtr = append((*taskQPtr), newQNode)
		}
	}
}

func ExecTask(flow *[]FlowTask, taskIdx int, taskQPtr *[]QNode, qIdx int, wgPtr *sync.WaitGroup) {
	var taskState State
	swapIdx := qIdx
	for {
		taskState = GetTaskState(flow, *taskQPtr, qIdx)
		swapIdx++
		if taskState == Ready && swapIdx < len(*taskQPtr) {
			// The deps not ready, swap it with the next one
			tmpQNode := (*taskQPtr)[qIdx]
			(*taskQPtr)[qIdx] = (*taskQPtr)[swapIdx]
			(*taskQPtr)[swapIdx] = tmpQNode
		} else {
			break
		}
	}

	(*flow)[taskIdx].State = taskState
	// fmt.Println("### Task name is:", (*flow)[taskIdx].Name)
	// fmt.Println("### Task state is:", taskState)
	if taskState == Running {
		// Handle foreach
		if (*flow)[taskIdx].Foreach != nil {
			// Fork all subnodes
			HandleForeach(flow, taskIdx, taskQPtr, qIdx, wgPtr)
		} else {
			// Render Params variables
			// FIXME: Add deps support
			if qIdx > 0 {
				baseTask := GetTaskByID(flow, (*taskQPtr)[qIdx].Bases[0])
				(*flow)[taskIdx].Params = RenderParams((*flow)[taskIdx].Params, (*flow)[0].Params, (*flow)[baseTask].Response, (*flow)[taskIdx].Meta)
			}
			(*flow)[taskIdx].Response = FakeAction((*flow)[taskIdx].Action, (*flow)[taskIdx].Params)
			// if (*flow)[taskIdx].Meta["foreach"] == nil {
			if Contains((*flow)[taskIdx].Meta["foreach"], "instance-1") {
				PrintTask(&(*flow)[taskIdx], "+")
			}
			(*flow)[taskIdx].State = Finished
		}

	} else {
		// if (*flow)[taskIdx].Meta["foreach"] == nil {
		if Contains((*flow)[taskIdx].Meta["txn"], "instance-1") {
			PrintTask(&(*flow)[taskIdx], "-")
		}
	}

	HandleOnXTasks(flow, taskIdx, taskQPtr, qIdx)
}

func CloneTaskQ(taskQ []QNode, qIdx int) []QNode {
	var subTaskQ []QNode

	for i := 0; i < qIdx; i++ {
		subTaskQ = append(subTaskQ, taskQ[i])
	}

	return subTaskQ
}

func HandleForeach(flow *[]FlowTask, taskIdx int, taskQPtr *[]QNode, qIdx int, wgPtr *sync.WaitGroup) {
	// Mark task in the main thread as skiped
	(*flow)[taskIdx].State = Skiped

	if qIdx > 0 {
		// FIXME: Mutilple bases (deps) support
		baseIdx := GetTaskByID(flow, (*taskQPtr)[qIdx].Bases[0])
		// Render foreach
		(*flow)[taskIdx].Foreach = RenderForeach((*flow)[taskIdx].Foreach, (*flow)[0].Params, (*flow)[baseIdx].Response, (*flow)[taskIdx].Meta)
	}

	// Fork all subnodes
	var newQueue []QNode
	for i := 0; i < GetForeachLen((*flow)[taskIdx].Foreach); i++ {
		// Fork one subnode, and add it into flow
		newTask := (*flow)[taskIdx]
		// Reinit UUID
		newTask.ID = GenerateUUID()
		// Reinit State
		newTask.State = Ready
		// Clone Params
		newTask.Params = CopyMap((*flow)[taskIdx].Params)
		// Inherit Meta
		metaForeach := make([]string, len((*flow)[taskIdx].Meta["foreach"]))
		copy(metaForeach, (*flow)[taskIdx].Meta["foreach"])
		metaTxn := make([]string, len((*flow)[taskIdx].Meta["txn"]))
		copy(metaTxn, (*flow)[taskIdx].Meta["txn"])
		// FIXME: Support more types for foreach
		// fmt.Println("Foreach:", GetForeachSlice((*flow)[taskIdx].Foreach, i))
		metaForeach = append(metaForeach, GetForeachSlice((*flow)[taskIdx].Foreach, i))
		newMeta := make(map[string][]string)
		newMeta["foreach"] = metaForeach
		newMeta["txn"] = metaTxn
		if qIdx > 0 {
			// FIXME: Mutilple bases (deps) support
			baseIdx := GetTaskByID(flow, (*taskQPtr)[qIdx].Bases[0])
			newMeta["txn"] = RenderTxn(newMeta["txn"], (*flow)[0].Params, (*flow)[baseIdx].Response, newMeta)
		} else {
			newMeta["txn"] = RenderTxn(newMeta["txn"], (*flow)[0].Params, (*flow)[0].Response, newMeta)
		}
		newTask.Meta = newMeta
		newTask.Foreach = nil

		// Fork new taskQueue
		*flow = append(*flow, newTask)
		newQNode := QNode{
			TaskID: newTask.ID,
			Bases:  (*taskQPtr)[qIdx].Bases,
			Subs:   nil,
		}
		newQueue = CloneTaskQ(*taskQPtr, qIdx)
		newQueue = append(newQueue, newQNode)

		// Execute tasks from the next node in new taskQueue
		(*wgPtr).Add(1)
		go ExecFlow(flow, newQueue, qIdx, wgPtr)
	}
}

func HandleOnXTasks(flow *[]FlowTask, taskIdx int, taskQPtr *[]QNode, qIdx int) {
	respCode := GetRespField((*flow)[taskIdx].Response, ".code")
	var forkOnXTaskIdxs []string
	var skipOnXTaskIdxs []string
	if respCode == "200" {
		// fmt.Println("Execute OnSuccess...")
		forkOnXTaskIdxs, skipOnXTaskIdxs = GetOnXTasks((*flow)[taskIdx].OnSuccess)
		skipOnXTaskIdxs = append(skipOnXTaskIdxs, GetOnXTasksString((*flow)[taskIdx].OnError)...)
	} else {
		// fmt.Println("Execute OnError...")
		forkOnXTaskIdxs, skipOnXTaskIdxs = GetOnXTasks((*flow)[taskIdx].OnError)
		skipOnXTaskIdxs = append(skipOnXTaskIdxs, GetOnXTasksString((*flow)[taskIdx].OnSuccess)...)
	}
	// fmt.Println("forkOnXTaskIdxs:", forkOnXTaskIdxs)
	// fmt.Println("skipOnXTaskIdxs:", skipOnXTaskIdxs)
	CloneOnXTasks(flow, taskIdx, taskQPtr, qIdx, forkOnXTaskIdxs)
	SkipOnXTasks(flow, taskIdx, taskQPtr, qIdx, skipOnXTaskIdxs)
}

func ExecFlow(flow *[]FlowTask, taskQueue []QNode, qIdx int, wgPtr *sync.WaitGroup) int {
	ret := 0
	curIdx := qIdx

	// Execute every task in the queue
	for {
		endIdx := len(taskQueue)
		// fmt.Println("*** curIdx:", curIdx, " | endIdx:", endIdx, "***")
		if curIdx == endIdx {
			break
		}

		// Main execution process
		taskIdx := GetTaskByID(flow, taskQueue[curIdx].TaskID)
		ExecTask(flow, taskIdx, &taskQueue, curIdx, wgPtr)

		curIdx++
	}
	(*wgPtr).Done()

	return ret
}
