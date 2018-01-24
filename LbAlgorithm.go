package s

import "net/http"

type LoadBalancer interface {

	// 每个请求完成后提供信息
	Response(node *NodeInfo, err error, response *http.Response, responseTimeing int64)

	// 请求时根据节点的得分取最小值发起请求
	Next(nodes []*NodeInfo, request *http.Request) *NodeInfo
}

type DefaultLoadBalancer struct{}

func (lba *DefaultLoadBalancer) Response(node *NodeInfo, err error, response *http.Response, responseTimeing int64) {
	node.Data = float64(node.UsedTimes) / float64(node.Weight)
}

func (lba *DefaultLoadBalancer) Next(nodes []*NodeInfo, request *http.Request) *NodeInfo {
	var minScore float64 = -1
	var minNode *NodeInfo = nil
	for _, node := range nodes {
		if node.Data == nil {
			node.Data = float64(node.UsedTimes) / float64(node.Weight)
		}
		score := node.Data.(float64)
		if minScore == -1 || score < minScore {
			minScore = score
			minNode = node
		}
	}
	return minNode
}
