package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zeromicro/go-zero/zrpc"
	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/client/chainsync"
)

func main() {
	// 创建RPC客户端连接
	// 注意：请根据您的实际配置修改地址
	rpcClientConfig := zrpc.RpcClientConf{
		Endpoints: []string{"localhost:9001"}, // 扫链服务地址
		Timeout:   5000,                       // 5秒超时
	}

	// 创建客户端
	client := chainsync.NewChainSync(zrpc.MustNewClient(rpcClientConfig))
	ctx := context.Background()

	fmt.Println("🔍 测试地址监控API接口")

	// 1. 添加地址监控
	fmt.Println("\n1️⃣ 添加地址监控")
	addAddrReq := &chainsync.AddAddressMonitorReq{
		Address: "0x1234567890123456789012345678901234567890", // 替换为实际地址
		Chain:   pb.BlockChainType_CHAIN_TYPE_ETHEREUM,        // 以太坊
		//Chain:  pb.BlockChainType_CHAIN_TYPE_BSC,              // 或者BSC
		//Chain:  pb.BlockChainType_CHAIN_TYPE_TRON,             // 或者TRON
	}

	addResp, err := client.AddAddressMonitor(ctx, addAddrReq)
	if err != nil {
		log.Printf("❌ 添加地址监控失败: %v", err)
	} else {
		fmt.Printf("✅ 添加地址监控成功: %+v\n", addResp)
		fmt.Printf("   监控ID: %s\n", addResp.MonitorId)
		fmt.Printf("   状态: %s\n", addResp.Message)
	}

	// 2. 获取监控地址列表
	fmt.Println("\n2️⃣ 获取监控地址列表")
	getMonitoredReq := &chainsync.GetMonitoredAddressesReq{
		Chain: pb.BlockChainType_CHAIN_TYPE_ETHEREUM, // 可以指定链类型，或留空获取所有
	}

	monitoredResp, err := client.GetMonitoredAddresses(ctx, getMonitoredReq)
	if err != nil {
		log.Printf("❌ 获取监控地址列表失败: %v", err)
	} else {
		fmt.Printf("✅ 获取监控地址列表成功\n")
		fmt.Printf("   总数: %d\n", monitoredResp.Total)
		fmt.Printf("   活跃数: %d\n", monitoredResp.ActiveCount)

		for i, monitor := range monitoredResp.Monitors {
			fmt.Printf("   [%d] 地址: %s, 链: %v, 状态: %t, 创建时间: %v\n",
				i+1, monitor.Address, monitor.Chain, monitor.Active, monitor.CreatedAt)
		}
	}

	// 3. 测试多链地址监控
	//fmt.Println("\n3️⃣ 添加多链地址监控")
	//chains := []pb.BlockChainType{
	//	pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
	//	pb.BlockChainType_CHAIN_TYPE_BSC,
	//	pb.BlockChainType_CHAIN_TYPE_TRON,
	//}

	//address := "TXYZ123456789012345678901234567890123456" // 示例TRON地址

	//for _, chain := range chains {
	//	addMultiReq := &chainsync.AddAddressMonitorReq{
	//		Address: address,
	//		Chain:   chain,
	//	}
	//
	//	resp, err := client.AddAddressMonitor(ctx, addMultiReq)
	//	if err != nil {
	//		log.Printf("❌ 添加 %s 链监控失败: %v", chain.String(), err)
	//	} else {
	//		fmt.Printf("✅ 成功添加 %s 链监控: %s\n", chain.String(), resp.Message)
	//	}
	//}

	// 等待一下
	time.Sleep(2 * time.Second)

	// 4. 移除地址监控
	fmt.Println("\n4️⃣ 移除地址监控")
	removeAddrReq := &chainsync.RemoveAddressMonitorReq{
		Address: "0x1234567890123456789012345678901234567890",
		Chain:   pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
	}

	removeResp, err := client.RemoveAddressMonitor(ctx, removeAddrReq)
	if err != nil {
		log.Printf("❌ 移除地址监控失败: %v", err)
	} else {
		fmt.Printf("✅ 移除地址监控成功: %+v\n", removeResp)
		fmt.Printf("   状态: %s\n", removeResp.Message)
	}

	// 5. 批量添加监控地址（模拟业务场景）
	//fmt.Println("\n5️⃣ 批量添加监控地址示例")
	//addresses := []string{
	//	"0xABCDEF1234567890123456789012345678901234",
	//	"0xFEDCBA0987654321098765432109876543210",
	//	"TXYZ1111111111111111111111111111111111111", // TRON地址
	//}
	//
	//for _, addr := range addresses {
	//	// 根据地址格式判断链类型
	//	var chain pb.BlockChainType
	//	if len(addr) == 42 && addr[:2] == "0x" {
	//		// 以太坊/BSC地址
	//		chain = pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	//	} else if len(addr) == 34 {
	//		// TRON地址
	//		chain = pb.BlockChainType_CHAIN_TYPE_TRON
	//	}
	//
	//	req := &chainsync.AddAddressMonitorReq{
	//		Address: addr,
	//		Chain:   chain,
	//	}
	//
	//	_, err := client.AddAddressMonitor(ctx, req)
	//	if err != nil {
	//		log.Printf("❌ 添加地址 %s 失败: %v", addr, err)
	//	} else {
	//		fmt.Printf("✅ 地址 %s 监控添加成功 (链: %s)\n", addr, chain.String())
	//	}
	//}

	// 6. 获取特定链的监控地址
	//fmt.Println("\n6️⃣ 获取TRON链监控地址")
	//getTronMonitoredReq := &chainsync.GetMonitoredAddressesReq{
	//	Chain: pb.BlockChainType_CHAIN_TYPE_TRON,
	//}
	//
	//tronMonitoredResp, err := client.GetMonitoredAddresses(ctx, getTronMonitoredReq)
	//if err != nil {
	//	log.Printf("❌ 获取TRON监控地址失败: %v", err)
	//} else {
	//	fmt.Printf("✅ TRON监控地址数量: %d\n", tronMonitoredResp.Total)
	//	for _, monitor := range tronMonitoredResp.Monitors {
	//		fmt.Printf("   地址: %s, 状态: %t\n", monitor.Address, monitor.Active)
	//	}
	//}
	//
}
