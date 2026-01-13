package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("=== 炸金花游戏 ===")
	fmt.Println("请选择模式:")
	fmt.Println("1. 启动服务器")
	fmt.Println("2. 交互式客户端")
	fmt.Print("请输入选项(1/2): ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	switch choice {
	case "1":
		runServer()
	case "2":
		runInteractiveClient()
	default:
		fmt.Println("无效的选项哦")
	}
}

// runServer 运行服务器
func runServer() {
	fmt.Println("=== 启动服务器 ===")

	// 创建服务器
	server := NewServer(":8080", 10)

	// 启动服务器
	if err := server.Start(); err != nil {
		fmt.Printf("服务器启动失败: %v\n", err)
		return
	}

	fmt.Println("服务器已启动，监听地址: :8080")
	fmt.Println("按 Ctrl+C 停止服务器")

	// 保持运行
	select {}
}

// runInteractiveClient 运行交互式客户端
func runInteractiveClient() {
	fmt.Println("=== 交互式客户端 ===")

	// 创建客户端
	client := NewGameClient()

	// 连接服务器
	fmt.Print("请输入服务器地址(默认 localhost:8080): ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	addr := strings.TrimSpace(scanner.Text())
	if addr == "" {
		addr = "localhost:8080"
	}

	if err := client.Connect(addr); err != nil {
		fmt.Printf("连接服务器失败: %v\n", err)
		return
	}

	// 提示玩家输入昵称
	fmt.Println("\n=== 欢迎来到炸金花游戏 ===")
	fmt.Println("请输入您的昵称以进入游戏大厅:")

	// 运行仪表盘模式
	client.RunDashboard()
}
