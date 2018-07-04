package discover

import (
	"net/http"
	"log"
)

type AppClient struct {
	excludes map[string]bool
	tryTimes int
}

func (appClient *AppClient) Next(app string, request *http.Request) *NodeInfo {
	return appClient.NextWithNode(app, "", request)
}

func (appClient *AppClient) NextWithNode(app, withNode string, request *http.Request) *NodeInfo {
	if appClient.excludes == nil {
		appClient.excludes = map[string]bool{}
	}

	if appNodes[app] == nil {
		log.Printf("DISCOVER	No App	%s", app)
		return nil
	}
	if len(appNodes[app]) == 0 {
		log.Printf("DISCOVER	No Node	%s	%d", app, len(appNodes[app]))
		return nil
	}

	appClient.tryTimes++
	if withNode != "" {
		appClient.excludes[withNode] = true
		return appNodes[app][withNode]
	}

	var node *NodeInfo
	nodes := make([]*NodeInfo, 0)
	for _, node := range appNodes[app] {
		if appClient.excludes[node.Addr] || node.FailedTimes >= config.CallRetryTimes {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		// 没有可用节点的情况下，尝试已经失败多次的节点
		for _, node := range appNodes[app] {
			if appClient.excludes[node.Addr] {
				continue
			}
			nodes = append(nodes, node)
		}
	}
	if len(nodes) > 0 {
		node = settedLoadBalancer.Next(nodes, request)
		appClient.excludes[node.Addr] = true
	}
	if node == nil {
		log.Printf("DISCOVER	No Node	%s	%d / %d", app, appClient.tryTimes, len(appNodes[app]))
	}

	return node
}
