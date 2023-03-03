package main

import "sync"

func main() {
	wg := sync.WaitGroup{}

	yml := ParseYml("createServer.yml")
	flow := InitFlow(yml)
	taskQueue := BuildTaskQueue(flow)
	wg.Add(1)
	ExecFlow(flow, taskQueue, 0, &wg)

	wg.Wait()
}
