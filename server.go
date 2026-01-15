package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket 升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// Client 客户端连接
type Client struct {
	Conn       *websocket.Conn // WebSocket连接
	Player     *Player         // 玩家信息
	ID         int             // 客户端ID
	IsOnline   bool            // 是否在线
	SendChan   chan Message    // 发送消息通道
	LastActive time.Time       // 最后活跃时间
}

// ClientManager 客户端管理器
type ClientManager struct {
	clients map[int]*Client // 客户端映射
	mu      sync.RWMutex    // 读写锁
	nextID  int             // 下一个客户端ID
}

// Server 服务器结构体
type Server struct {
	Manager     *ClientManager // 客户端管理器
	Addr        string         // 监听地址
	Rooms       map[int]*Room  // 房间映射
	NextRoomID  int            // 下一个房间ID
	RoomCounter int            // 房间计数器
	Ante        int            // 底注金额
	mu          sync.RWMutex   // 读写锁
}

// NewServer 创建新服务器
func NewServer(addr string, ante int) *Server {
	return &Server{
		Manager: &ClientManager{
			clients: make(map[int]*Client),
			nextID:  1,
		},
		Addr:       addr,
		Rooms:      make(map[int]*Room),
		NextRoomID: 1,
		Ante:       ante,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	// 设置HTTP路由
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("炸金花游戏服务器运行中..."))
	})

	http.HandleFunc("/ws", s.handleWebSocket)

	fmt.Printf("服务器启动,监听地址: %s\n", s.Addr)
	return http.ListenAndServe(s.Addr, nil)
}

// handleWebSocket 处理WebSocket连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("升级WebSocket失败: %v\n", err)
		return
	}

	client := &Client{
		Conn:       conn,
		ID:         s.Manager.nextID,
		IsOnline:   true,
		SendChan:   make(chan Message, 100),
		LastActive: time.Now(),
	}
	s.Manager.nextID++

	s.Manager.AddClient(client)

	fmt.Printf("新客户端连接: %s, ID: %d\n", conn.RemoteAddr(), client.ID)

	// 启动发送消息的协程
	go client.writeLoop()

	// 启动处理消息的协程
	go s.handleClient(client)
}

// handleClient 处理客户端消息
func (s *Server) handleClient(client *Client) {
	defer func() {
		client.IsOnline = false
		close(client.SendChan)

		// 如果玩家在游戏中，处理离开逻辑
		if client.Player != nil && client.Player.Room != nil {
			room := client.Player.Room
			player := client.Player

			// 如果游戏正在进行
			if room.GameState == StatePlaying {
				// 如果该玩家是当前操作者，触发 Fold 并切换到下一个人
				if room.Players[room.TurnIndex].ID == player.ID {
					room.HandleAction(player, ActionFold, nil)
				} else {
					// 不是当前操作者，直接标记为弃牌
					player.Fold()
				}

				// 检查是否只剩一个活跃玩家
				activeCount := 0
				for _, p := range room.Players {
					if p.Status != StatusFolded && p.Status != StatusLost {
						activeCount++
					}
				}
				if activeCount <= 1 {
					room.Settling()
				}
			}

			// 从房间移除玩家
			room.RemovePlayer(player.ID)

			// 检查房间是否为空
			if room.GetPlayerCount() == 0 {
				s.RemoveRoom(room.ID)
				fmt.Printf("房间 %d 已销毁\n", room.ID)
			} else {
				// 广播玩家离开消息，只向房间内的玩家发送
				s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 离开了游戏", player.Name), nil), room.ID)
			}
		}

		client.Conn.Close()
		s.Manager.RemoveClient(client.ID)
		fmt.Printf("客户端断开连接: ID %d\n", client.ID)
	}()

	// 启动心跳检测协程
	go s.heartbeatCheck(client)

	for {
		// 读取JSON消息
		var msg Message
		err := client.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("读取消息失败: %v\n", err)
			}
			break
		}

		// 更新最后活跃时间
		client.LastActive = time.Now()

		s.handleMessage(client, msg)
	}
}

// writeLoop 写入消息循环(异步发送)
func (c *Client) writeLoop() {
	for msg := range c.SendChan {
		if !c.IsOnline {
			break
		}

		err := c.Conn.WriteJSON(msg)
		if err != nil {
			fmt.Printf("发送消息失败: %v\n", err)
			c.IsOnline = false
			break
		}
	}
}

// heartbeatCheck 心跳检测
func (s *Server) heartbeatCheck(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !client.IsOnline {
			break
		}

		// 检查60秒内是否有活动
		if time.Since(client.LastActive) > 60*time.Second {
			fmt.Printf("客户端 %d 超时断开\n", client.ID)
			client.IsOnline = false
			client.Conn.Close()
			s.Manager.RemoveClient(client.ID)
			break
		}
	}
}

// handleMessage 处理消息
func (s *Server) handleMessage(client *Client, msg Message) {
	switch msg.Action {
	case ActionLogin:
		s.handleLogin(client, msg)
	case ActionChat:
		s.handleChat(client, msg)
	case ActionBet:
		s.handleBet(client, msg)
	case ActionCall:
		s.handleCall(client)
	case ActionFold:
		s.handleFold(client)
	case ActionCheck:
		s.handleCheck(client)
	case ActionAllIn:
		s.handleAllIn(client)
	case ActionCompare:
		s.handleCompare(client, msg)
	case ActionPing:
		// 心跳包,回复响应
		client.LastActive = time.Now()
		s.sendResponse(client, true, "pong", nil)
	case ActionLeave:
		if client.Player.Room != nil {
			room := client.Player.Room

			// 检查是否是房主退出
			isMasterLeaving := room.MasterID == client.Player.ID
			leftPlayerName := client.Player.Name

			room.RemovePlayer(client.Player.ID)
			client.Player.Room = nil

			// 如果房间没有玩家了，直接删除房间
			if room.GetPlayerCount() == 0 {
				s.RemoveRoom(room.ID)
				// 1. 给退出的玩家发一个确认响应，包含 room_id=0 表示回到大厅
				client.SendChan <- NewMessage(ActionResponse, map[string]interface{}{
					"success": true,
					"message": "已退出房间",
					"data": map[string]interface{}{
						"room_id": 0, // 告诉客户端已退出房间，回到大厅
					},
				})
				return
			}

			// 如果房主退出且房间还有玩家，转移房主
			if isMasterLeaving {
				room.MasterID = room.Players[0].ID
				// 遍历所有客户端，找到在同一房间的其他客户端发送消息
				s.Manager.mu.RLock()
				for _, c := range s.Manager.clients {
					if c.Player != nil && c.Player.Room != nil && c.Player.Room.ID == room.ID {
						c.SendChan <- NewMessage(ActionResponse, map[string]interface{}{
							"success": true,
							"message": fmt.Sprintf("房主 %s 已退出，新房主为 %s", leftPlayerName, room.Players[0].Name),
						})
					}
				}
				s.Manager.mu.RUnlock()
			}

			// 2. 给退出的玩家发一个确认响应
			client.SendChan <- NewMessage(ActionResponse, map[string]interface{}{
				"success": true,
				"message": "已退出房间",
				"data": map[string]interface{}{
					"room_id": room.ID, // 告诉客户端仍在房间里
				},
			})

			// 3. 给房间剩下的人发广播
			s.broadcastGameUpdate(room)
		}
	case ActionListRooms:
		s.handleListRooms(client)
	case ActionCreateRoom:
		s.handleCreateRoom(client, msg)
	case ActionJoinRoom:
		s.handleJoinRoom(client, msg)
	case ActionStartGame:
		s.handleStartGame(client)
	default:
		s.sendError(client, "未知的动作类型")
	}
}

// handleLogin 处理登录
func (s *Server) handleLogin(client *Client, msg Message) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		s.sendError(client, "无效的登录载荷")
		return
	}

	name, ok := payload["name"].(string)
	if !ok || name == "" {
		s.sendError(client, "玩家名称不能为空")
		return
	}

	// 创建新玩家
	player := NewPlayer(client.ID, name, 1000)
	client.Player = player

	// 发送登录成功响应（默认进入大厅，RoomID = 0）
	s.sendResponse(client, true, "登录成功，进入大厅", map[string]interface{}{
		"player": PlayerInfo{
			ID:     player.ID,
			Name:   player.Name,
			Chips:  player.Chips,
			Status: player.Status,
		},
		"room_id": 0,
	})
}

// handleChat 处理聊天
func (s *Server) handleChat(client *Client, msg Message) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		s.sendError(client, "无效的聊天载荷")
		return
	}

	content, ok := payload["content"].(string)
	if !ok || content == "" {
		s.sendError(client, "聊天内容不能为空")
		return
	}

	// 根据玩家所在位置广播聊天消息
	chatMsg := NewBroadcastMessage("chat", content, map[string]interface{}{
		"player_id":   client.Player.ID,
		"player_name": client.Player.Name,
	})

	if client.Player.Room != nil && client.Player.Room.ID != 0 {
		// 在房间内，只向房间内的玩家广播
		s.Manager.BroadcastToRoom(chatMsg, client.Player.Room.ID)
	} else {
		// 在大厅，只向大厅的玩家广播
		s.Manager.BroadcastToLobby(chatMsg)
	}
}

// handleBet 处理下注（闷注/加注）
func (s *Server) handleBet(client *Client, msg Message) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	room := client.Player.Room
	if room == nil {
		s.sendError(client, "玩家不在任何房间")
		return
	}

	// 检查玩家是否已弃牌
	if client.Player.Status == StatusFolded || client.Player.Status == StatusLost {
		s.sendError(client, "你已弃牌，无法进行操作")
		return
	}

	// 指令拦截器：校验是否轮到该玩家
	// fmt.Printf("[DEBUG] 回合检查: 当前回合玩家ID=%d, 请求玩家ID=%d\n", room.Players[room.TurnIndex].ID, client.Player.ID)
	if room.Players[room.TurnIndex].ID != client.Player.ID {
		currentPlayer := room.Players[room.TurnIndex]
		s.sendError(client, fmt.Sprintf("还没轮到你，当前是 %s 的回合", currentPlayer.Name))
		return
	}

	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		s.sendError(client, "无效的下注载荷")
		return
	}

	amountFloat, ok := payload["amount"].(float64)
	if !ok {
		s.sendError(client, "无效的下注金额")
		return
	}

	// 执行下注
	if err := room.HandleAction(client.Player, ActionBet, amountFloat); err != nil {
		s.sendError(client, err.Error())
		return
	}

	// 根据玩家状态返回不同的成功消息
	if client.Player.Status == StatusChecked {
		s.sendResponse(client, true, "加注成功", map[string]interface{}{
			"chips":     client.Player.Chips,
			"round_bet": client.Player.RoundBet,
		})
		// 广播加注消息给房间内其他玩家
		s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 加注 %d", client.Player.Name, int(amountFloat)), nil), room.ID)
	} else {
		s.sendResponse(client, true, "闷注成功", map[string]interface{}{
			"chips":     client.Player.Chips,
			"round_bet": client.Player.RoundBet,
		})
		// 广播闷注消息给房间内其他玩家
		s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 闷注 %d", client.Player.Name, int(amountFloat)), nil), room.ID)
	}

	// 广播游戏更新
	s.broadcastGameUpdate(room)
}

// handleCall 处理跟注
func (s *Server) handleCall(client *Client) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	room := client.Player.Room
	if room == nil {
		s.sendError(client, "玩家不在任何房间")
		return
	}

	// 检查玩家是否已弃牌
	if client.Player.Status == StatusFolded || client.Player.Status == StatusLost {
		s.sendError(client, "你已弃牌，无法进行操作")
		return
	}

	// 指令拦截器：校验是否轮到该玩家
	if room.Players[room.TurnIndex].ID != client.Player.ID {
		currentPlayer := room.Players[room.TurnIndex]
		s.sendError(client, fmt.Sprintf("还没轮到你，当前是 %s 的回合", currentPlayer.Name))
		return
	}

	// 首局首位行动者不能跟注，只能闷注或弃牌
	if room.RoundCount == 0 && room.TurnIndex == 0 {
		if client.Player.Status == StatusChecked {
			s.sendError(client, "首位行动者不能跟注，请使用加注(r)或弃牌(f)")
		} else {
			s.sendError(client, "首位行动者不能跟注，请使用闷注(b)或弃牌(f)")
		}
		return
	}

	// 计算跟注金额
	var callAmount int
	if client.Player.Status == StatusChecked {
		// 已看牌玩家：跟注 CurrentSingleBet
		callAmount = room.CurrentSingleBet
	} else {
		// 未看牌玩家：跟注 CurrentSingleBet / 2
		callAmount = room.CurrentSingleBet / 2
	}

	// 检查筹码是否足够
	if callAmount > client.Player.Chips {
		s.sendError(client, fmt.Sprintf("筹码不足，跟注需要 %d 筹码。请选择弃牌(f)或全押", callAmount))
		return
	}

	// 执行跟注
	if err := room.HandleAction(client.Player, ActionCall, nil); err != nil {
		s.sendError(client, err.Error())
		return
	}

	s.sendResponse(client, true, "跟注成功", map[string]interface{}{
		"chips":     client.Player.Chips,
		"round_bet": client.Player.RoundBet,
	})

	// 广播跟注消息给房间内其他玩家
	s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 跟注 %d", client.Player.Name, callAmount), nil), room.ID)

	// 广播游戏更新
	s.broadcastGameUpdate(room)
}

// handleFold 处理弃牌
func (s *Server) handleFold(client *Client) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	room := client.Player.Room
	if room == nil {
		s.sendError(client, "玩家不在任何房间")
		return
	}

	// 指令拦截器：校验是否轮到该玩家
	if room.Players[room.TurnIndex].ID != client.Player.ID {
		currentPlayer := room.Players[room.TurnIndex]
		s.sendError(client, fmt.Sprintf("还没轮到你，当前是 %s 的回合", currentPlayer.Name))
		return
	}

	// 调用 room 的 handleFold 处理后续逻辑（检查剩余玩家、结算等）
	if err := room.HandleAction(client.Player, ActionFold, nil); err != nil {
		s.sendError(client, err.Error())
		return
	}

	s.sendResponse(client, true, "弃牌成功", nil)

	// 广播弃牌消息给房间内其他玩家
	s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 弃牌", client.Player.Name), nil), room.ID)

	// 广播游戏更新
	s.broadcastGameUpdate(room)
}

// handleCheck 处理看牌
func (s *Server) handleCheck(client *Client) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	room := client.Player.Room
	if room == nil {
		s.sendError(client, "玩家不在任何房间")
		return
	}

	// 检查玩家是否已弃牌
	if client.Player.Status == StatusFolded || client.Player.Status == StatusLost {
		s.sendError(client, "你已弃牌，无法进行操作")
		return
	}

	// 指令拦截器：校验是否轮到该玩家
	if room.Players[room.TurnIndex].ID != client.Player.ID {
		currentPlayer := room.Players[room.TurnIndex]
		s.sendError(client, fmt.Sprintf("还没轮到你，当前是 %s 的回合", currentPlayer.Name))
		return
	}

	client.Player.CheckCards()

	// 发送手牌给该玩家，格式化为客户端期望的格式
	cardsData := make([]map[string]interface{}, 0, len(client.Player.Cards))
	for _, card := range client.Player.Cards {
		cardsData = append(cardsData, map[string]interface{}{
			"suit": int(card.Suit), // 发送花色的数字值 (0=黑桃, 1=红桃, 2=方块, 3=梅花)
			"rank": int(card.Rank), // 发送点数的数字值
		})
	}

	s.sendResponse(client, true, "看牌成功", map[string]interface{}{
		"cards": cardsData,
	})

	// 广播看牌消息给房间内其他玩家
	s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 看牌成功", client.Player.Name), nil), room.ID)

	// 广播游戏更新(不包含手牌)
	s.broadcastGameUpdate(room)
}

// handleAllIn 处理全押
func (s *Server) handleAllIn(client *Client) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	room := client.Player.Room
	if room == nil {
		s.sendError(client, "玩家不在任何房间")
		return
	}

	// 检查玩家是否已弃牌
	if client.Player.Status == StatusFolded || client.Player.Status == StatusLost {
		s.sendError(client, "你已弃牌，无法进行操作")
		return
	}

	// 指令拦截器：校验是否轮到该玩家
	if room.Players[room.TurnIndex].ID != client.Player.ID {
		currentPlayer := room.Players[room.TurnIndex]
		s.sendError(client, fmt.Sprintf("还没轮到你，当前是 %s 的回合", currentPlayer.Name))
		return
	}

	// 执行全押
	if err := room.HandleAction(client.Player, ActionAllIn, nil); err != nil {
		s.sendError(client, err.Error())
		return
	}

	s.sendResponse(client, true, "全押成功", map[string]interface{}{
		"chips":     client.Player.Chips,
		"round_bet": client.Player.RoundBet,
	})

	// 广播全押消息给房间内其他玩家
	s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 全押", client.Player.Name), nil), room.ID)

	// 广播游戏更新
	s.broadcastGameUpdate(room)
}

// handleCompare 处理比牌
func (s *Server) handleCompare(client *Client, msg Message) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	// fmt.Printf("[DEBUG] 服务器收到比牌请求，玩家ID=%d\n", client.Player.ID)

	room := client.Player.Room
	if room == nil {
		s.sendError(client, "玩家不在任何房间")
		return
	}

	// 指令拦截器：校验是否轮到该玩家
	if room.Players[room.TurnIndex].ID != client.Player.ID {
		currentPlayer := room.Players[room.TurnIndex]
		s.sendError(client, fmt.Sprintf("还没轮到你，当前是 %s 的回合", currentPlayer.Name))
		return
	}

	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		s.sendError(client, "无效的比牌载荷")
		return
	}
	// fmt.Printf("[DEBUG] 比牌payload: %v\n", payload)

	targetIDFloat, ok := payload["target_id"].(float64)
	if !ok {
		s.sendError(client, "无效的目标玩家ID")
		return
	}

	targetID := int(targetIDFloat)
	target := room.GetPlayer(targetID)
	if target == nil {
		s.sendError(client, "目标玩家不存在")
		return
	}

	// 校验：发起者和目标必须都在房间内且未弃牌
	if client.Player.Status == StatusFolded || client.Player.Status == StatusLost {
		s.sendError(client, "你已弃牌或已出局，无法比牌")
		return
	}

	if target.Status == StatusFolded || target.Status == StatusLost {
		s.sendError(client, "目标玩家已弃牌或已出局")
		return
	}

	// 校验：不能和自己比牌
	if targetID == client.Player.ID {
		s.sendError(client, "不能和自己比牌")
		return
	}

	// 费用扣除：比牌费用等于当前跟注金额
	// 当前跟注：已看牌玩家支付 CurrentSingleBet，闷牌玩家支付 CurrentSingleBet / 2
	compareCost := room.CurrentSingleBet
	if client.Player.Status != StatusChecked {
		compareCost = room.CurrentSingleBet / 2
	}

	// 检查余额是否足够
	if client.Player.Chips < compareCost {
		s.sendError(client, fmt.Sprintf("筹码不足，比牌需要 %d 筹码", compareCost))
		return
	}

	// 扣除比牌费用
	client.Player.Chips -= compareCost
	client.Player.RoundBet += compareCost
	room.Pot += compareCost

	// 先判断谁是输家，记录结果，避免后续操作清空手牌导致比较结果错误
	result := CompareHands(client.Player, target)
	var loserName string
	if result == 1 {
		loserName = target.Name
	} else if result == -1 {
		loserName = client.Player.Name
	} else {
		loserName = client.Player.Name
	}

	// 调试日志：打印比牌结果
	fmt.Printf("[DEBUG] 比牌结果: %s vs %s, result=%d, loser=%s\n",
		client.Player.Name, target.Name, result, loserName)

	// 调用房间处理比牌逻辑
	// fmt.Printf("[DEBUG] 调用room.HandleAction进行比牌\n")
	if err := room.HandleAction(client.Player, ActionCompare, payload); err != nil {
		// fmt.Printf("[DEBUG] room.HandleAction返回错误: %v\n", err)
		s.sendError(client, err.Error())
		return
	}
	// fmt.Printf("[DEBUG] room.HandleAction调用成功\n")

	// 广播比牌结果（不包含手牌内容），只向房间内的玩家发送
	broadcastContent := fmt.Sprintf("玩家 %s 发起比牌，玩家 %s 战败出局！", client.Player.Name, loserName)
	// fmt.Printf("[DEBUG] 服务器准备广播比牌结果: %s\n", broadcastContent)
	s.Manager.BroadcastToRoom(NewBroadcastMessage("system", broadcastContent, nil), room.ID)

	s.sendResponse(client, true, "比牌成功", nil)

	// 广播游戏更新
	s.broadcastGameUpdate(room)
}

// sendResponse 发送响应
func (s *Server) sendResponse(client *Client, success bool, message string, data interface{}) {
	response := NewResponse(success, message, data)
	// 将Response包装为Message
	msg := NewMessage("response", response)

	// 静默处理连接错误
	select {
	case client.SendChan <- msg:
	default:
		// 发送失败，客户端可能已断开
	}
}

// sendError 发送错误响应
func (s *Server) sendError(client *Client, message string) {
	s.sendResponse(client, false, message, nil)
}

// handleListRooms 处理列出房间
func (s *Server) handleListRooms(client *Client) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rooms := make([]RoomBrief, 0, len(s.Rooms))
	for _, room := range s.Rooms {
		masterName := "未知"
		for _, player := range room.Players {
			if player.ID == room.MasterID {
				masterName = player.Name
				break
			}
		}

		status := "等待中"
		if room.GameState == StatePlaying {
			status = "游戏中"
		} else if room.GameState == StateSettling {
			status = "结算中"
		}

		rooms = append(rooms, RoomBrief{
			ID:          room.ID,
			MasterID:    room.MasterID,
			MasterName:  masterName,
			PlayerCount: room.GetPlayerCount(),
			Status:      status,
		})
	}

	// 发送房间列表给请求的客户端
	s.sendResponse(client, true, "房间列表", map[string]interface{}{
		"rooms": rooms,
	})
}

// handleCreateRoom 处理创建房间
func (s *Server) handleCreateRoom(client *Client, msg Message) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	// 创建新房间
	room := NewRoom(s.RoomCounter + 1)
	room.MasterID = client.Player.ID

	s.mu.Lock()
	s.RoomCounter++
	s.Rooms[room.ID] = room
	s.mu.Unlock()

	// 加入房间
	if room.AddPlayer(client.Player) {
		client.Player.Room = room

		s.sendResponse(client, true, "创建房间成功", map[string]interface{}{
			"room_id": room.ID,
		})

		// 只向大厅的玩家广播房间创建消息
		s.Manager.BroadcastToLobby(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 创建了房间 %d", client.Player.Name, room.ID), nil))
		s.broadcastGameUpdate(room)
	} else {
		s.sendError(client, "创建房间失败")
	}
}

// handleJoinRoom 处理加入房间
func (s *Server) handleJoinRoom(client *Client, msg Message) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		s.sendError(client, "无效的加入房间载荷")
		return
	}

	roomIDFloat, ok := payload["room_id"].(float64)
	if !ok {
		s.sendError(client, "无效的房间ID")
		return
	}

	roomID := int(roomIDFloat)

	s.mu.RLock()
	room, exists := s.Rooms[roomID]
	s.mu.RUnlock()

	if !exists {
		s.sendError(client, "房间不存在")
		return
	}

	if room.GameState == StatePlaying {
		s.sendError(client, "房间游戏中，无法加入")
		return
	}

	if room.GetPlayerCount() >= 6 {
		s.sendError(client, "房间已满")
		return
	}

	// 加入房间
	if room.AddPlayer(client.Player) {
		client.Player.Room = room

		s.sendResponse(client, true, "加入房间成功", map[string]interface{}{
			"room_id": room.ID,
		})

		// 只向房间内的玩家广播加入消息
		s.Manager.BroadcastToRoom(NewBroadcastMessage("system", fmt.Sprintf("玩家 %s 加入了房间 %d", client.Player.Name, room.ID), nil), room.ID)
		s.broadcastGameUpdate(room)
	} else {
		s.sendError(client, "加入房间失败")
	}
}

// handleStartGame 处理开始游戏
func (s *Server) handleStartGame(client *Client) {
	if client.Player == nil {
		s.sendError(client, "请先登录")
		return
	}

	room := client.Player.Room
	if room == nil {
		s.sendError(client, "玩家不在任何房间")
		return
	}

	// 检查是否为房主
	if room.MasterID != client.Player.ID {
		s.sendError(client, "只有房主可以开始游戏")
		return
	}

	// 检查游戏状态
	if room.GameState != StateWaiting {
		s.sendError(client, "游戏不在等待状态")
		return
	}

	// 检查玩家数量
	if room.GetPlayerCount() < 2 {
		s.sendError(client, "至少需要2名玩家才能开始游戏")
		return
	}

	// 开始游戏
	room.StartGame()

	s.sendResponse(client, true, "游戏开始", nil)
	// 只向房间内的玩家广播游戏开始消息
	s.Manager.BroadcastToRoom(NewBroadcastMessage("system", "游戏开始！", nil), room.ID)
	s.broadcastGameUpdate(room)
}

// AddClient 添加客户端
func (cm *ClientManager) AddClient(client *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.clients[client.ID] = client
}

// RemoveClient 移除客户端
func (cm *ClientManager) RemoveClient(id int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.clients, id)
}

// GetClient 获取客户端
func (cm *ClientManager) GetClient(id int) (*Client, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	client, ok := cm.clients[id]
	return client, ok
}

// GetAllClients 获取所有客户端
func (cm *ClientManager) GetAllClients() []*Client {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	clients := make([]*Client, 0, len(cm.clients))
	for _, client := range cm.clients {
		clients = append(clients, client)
	}
	return clients
}

// Broadcast 广播消息给所有客户端
func (cm *ClientManager) Broadcast(msg BroadcastMessage) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, client := range cm.clients {
		if client.IsOnline {
			// 将BroadcastMessage包装为Message
			wrappedMsg := NewMessage(ActionSystem, BroadcastMessage{
				Type:    msg.Type,
				Content: msg.Content,
				Data:    msg.Data,
			})
			// 异步发送,避免阻塞
			select {
			case client.SendChan <- wrappedMsg:
			default:
				// 发送通道已满,跳过该客户端
			}
		}
	}
}

// BroadcastToRoom 广播消息给指定房间的玩家
func (cm *ClientManager) BroadcastToRoom(msg BroadcastMessage, roomID int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, client := range cm.clients {
		if client.IsOnline && client.Player != nil && client.Player.Room != nil && client.Player.Room.ID == roomID {
			// 将BroadcastMessage包装为Message
			wrappedMsg := NewMessage(ActionSystem, BroadcastMessage{
				Type:    msg.Type,
				Content: msg.Content,
				Data:    msg.Data,
			})
			// 异步发送,避免阻塞
			select {
			case client.SendChan <- wrappedMsg:
			default:
				// 发送通道已满,跳过该客户端
			}
		}
	}
}

// BroadcastToLobby 广播消息给大厅中的玩家
func (cm *ClientManager) BroadcastToLobby(msg BroadcastMessage) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, client := range cm.clients {
		if client.IsOnline && client.Player != nil && (client.Player.Room == nil || client.Player.Room.ID == 0) {
			// 将BroadcastMessage包装为Message
			wrappedMsg := NewMessage(ActionSystem, BroadcastMessage{
				Type:    msg.Type,
				Content: msg.Content,
				Data:    msg.Data,
			})
			// 异步发送,避免阻塞
			select {
			case client.SendChan <- wrappedMsg:
			default:
				// 发送通道已满,跳过该客户端
			}
		}
	}
}

// AssignRoom 分配房间
func (s *Server) AssignRoom(client *Client) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 查找有空位的房间
	for _, room := range s.Rooms {
		if room.GetPlayerCount() < 6 {
			if room.AddPlayer(client.Player) {
				client.Player.Room = room
				return room
			}
		}
	}

	// 没有空位房间,创建新房间
	room := NewRoom(s.NextRoomID)
	s.NextRoomID++
	s.Rooms[room.ID] = room

	if room.AddPlayer(client.Player) {
		client.Player.Room = room
		return room
	}

	return nil
}

// GetRoom 获取房间
func (s *Server) GetRoom(roomID int) *Room {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Rooms[roomID]
}

// RemoveRoom 移除房间
func (s *Server) RemoveRoom(roomID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Rooms, roomID)
}

// GetAvailableRoom 获取可用房间
func (s *Server) GetAvailableRoom() *Room {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, room := range s.Rooms {
		if room.GetPlayerCount() < 6 {
			return room
		}
	}
	return nil
}

// broadcastGameUpdate 广播游戏更新
func (s *Server) broadcastGameUpdate(room *Room) {
	roomInfo := room.GetRoomInfo()
	s.Manager.BroadcastToRoom(NewBroadcastMessage("game_update", "游戏状态更新", roomInfo), room.ID)
}

// Stop 停止服务器
func (s *Server) Stop() error {
	// 简单的HTTP服务不需要手动Close，直接返回nil
	return nil
}
