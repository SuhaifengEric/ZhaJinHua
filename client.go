package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MaxLogSize = 8 // 最大日志数量
)

// padding 字符串补全到指定长度
func padding(str string, length int) string {
	runes := []rune(str)
	if len(runes) >= length {
		return string(runes[:length])
	}
	return str + strings.Repeat(" ", length-len(runes))
}

// GameClient 游戏客户端
type GameClient struct {
	Conn          *websocket.Conn // WebSocket连接
	IsOnline      bool            // 是否在线
	PlayerID      int             // 玩家ID
	MsgLog        []string        // 消息日志
	GameInfo      GameInfo        // 游戏信息
	MyPlayer      *PlayerInfo     // 我的玩家信息
	MyCards       []Card          // 我的手牌
	LastPing      time.Time       // 最后心跳时间
	IsExiting     bool            // 是否正在退出房间（等待服务器响应）
	HasChecked    bool            // 是否已经看过牌（持久化标记）
	LastGameState string          // 上一局游戏状态，用于判断新游戏开始
	LastWinAmount int             // 上一次获胜金额，用于防止重复打印游戏结束消息
	drawTimer     *time.Timer     // 延迟绘制定时器
}

// NewGameClient 创建新客户端
func NewGameClient() *GameClient {
	return &GameClient{
		IsOnline: false,
		MsgLog:   make([]string, 0, MaxLogSize),
	}
}

// AddLog 添加消息日志
func (c *GameClient) AddLog(msg string) {
	c.MsgLog = append(c.MsgLog, msg)
	if len(c.MsgLog) > MaxLogSize {
		c.MsgLog = c.MsgLog[1:]
	}
}

// drawLobby 绘制大厅视图
func (c *GameClient) drawLobby() {
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("  游戏大厅")
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println("  指令: ls(刷新) n(创建房间) j id(加入) exit(退出)")
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println("  消息记录")
	fmt.Println("────────────────────────────────────────────────────────────────")

	for _, msg := range c.MsgLog {
		fmt.Printf("  %s\n", msg)
	}

	for i := len(c.MsgLog); i < MaxLogSize; i++ {
		fmt.Println()
	}
}

// DrawTable 绘制游戏界面
func (c *GameClient) DrawTable() {
	// 取消之前的定时器
	if c.drawTimer != nil {
		c.drawTimer.Stop()
	}

	// 启动新的定时器，50ms后绘制
	// 这样可以在短时间内收到多条消息时只绘制一次，同时保持响应速度
	c.drawTimer = time.AfterFunc(50*time.Millisecond, func() {
		c.doDrawTable()
	})
}

// doDrawTable 实际执行绘制
func (c *GameClient) doDrawTable() {
	c.drawTimer = nil
	fmt.Print("\033[H\033[2J")

	// 如果不在房间中，显示大厅视图
	if c.GameInfo.RoomID == 0 {
		c.drawLobby()
		return
	}

	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Printf("  房间: %d  │  奖池: %d  │  底注: %d\n", c.GameInfo.RoomID, c.GameInfo.Pot, c.GameInfo.Ante)
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println("  玩家列表")
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println("  指示 │ ID │ 玩家名             │ 筹码       │ 下注       │ 状态      ")
	fmt.Println("────────────────────────────────────────────────────────────────")

	for _, player := range c.GameInfo.Players {
		cursor := "  "
		if player.ID == c.GameInfo.CurrentTurn {
			cursor = ">>>"
		}

		displayName := player.Name
		if player.ID == c.GameInfo.MasterID {
			displayName = fmt.Sprintf("%s(M)", displayName)
		}
		if player.ID == c.GameInfo.LastWinnerID {
			displayName = fmt.Sprintf("%s(W)", displayName)
		}

		statusText := getStatusName(player.Status)

		fmt.Printf("  %-4s │ %-4d │ %-18s │ %-10d │ %-10d │ %-10s\n",
			cursor, player.ID, displayName, player.Chips, player.RoundBet, statusText)
	}

	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println("  消息记录")
	fmt.Println("────────────────────────────────────────────────────────────────")

	for _, msg := range c.MsgLog {
		fmt.Printf("  %s\n", msg)
	}

	for i := len(c.MsgLog); i < MaxLogSize; i++ {
		fmt.Println()
	}

	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println("  操作提示")
	fmt.Println("────────────────────────────────────────────────────────────────")

	if c.MyPlayer != nil {
		// 只在游戏进行中显示手牌和跟注信息
		if c.GameInfo.GameStatus == "playing" {
			if len(c.MyCards) > 0 {
				cardsStr := ""
				for _, card := range c.MyCards {
					cardsStr += card.String() + " "
				}
				fmt.Printf("  手牌: %s\n", cardsStr)
			} else {
				fmt.Println("  手牌: 未看牌")
			}

			// 根据玩家是否看牌动态计算当前跟注金额
			// CurrentSingleBet 统一表示“已看牌玩家的跟注标准”
			// 已看牌玩家：当前跟注 = CurrentSingleBet
			// 闷牌玩家：当前跟注 = CurrentSingleBet / 2
			displayBet := c.GameInfo.CurrentSingleBet
			if c.MyPlayer != nil && c.MyPlayer.Status != StatusChecked {
				displayBet = c.GameInfo.CurrentSingleBet / 2
			}

			fmt.Printf("  当前跟注: %d\n", displayBet)

			// UI回合锁定：若不是自己的回合，显示等待状态
			if c.GameInfo.CurrentTurn != c.MyPlayer.ID {
				currentPlayerName := "未知"
				for _, player := range c.GameInfo.Players {
					if player.ID == c.GameInfo.CurrentTurn {
						currentPlayerName = player.Name
						break
					}
				}
				fmt.Printf("  [等待中] 当前轮到 %s 行动...\n", currentPlayerName)
			}
		} else {
			// 游戏未开始，根据是否为房主显示不同提示
			if c.MyPlayer.ID == c.GameInfo.MasterID {
				fmt.Println("  请输入快捷键 s 开始游戏")
			} else {
				fmt.Println("  等待房主开始游戏...")
			}
		}
	} else {
		fmt.Println("  请先登录")
	}

	fmt.Println("────────────────────────────────────────────────────────────────")

	// 根据玩家状态动态显示指令提示
	if c.MyPlayer != nil && c.GameInfo.GameStatus == "playing" {
		// 只有轮到玩家时才显示操作提示
		if c.GameInfo.CurrentTurn == c.MyPlayer.ID {
			if c.MyPlayer.Status == StatusChecked {
				// 已看牌玩家
				fmt.Println("  快捷键: [r 金额]加注 [c]跟注 [f]弃牌 [v id]比牌 [exit]退出")
			} else {
				// 未看牌玩家
				fmt.Println("  快捷键: [k]看牌 [b 金额]闷注 [c]跟注 [f]弃牌 [v id]比牌 [exit]退出")
			}
		} else {
			// 不是玩家回合，显示等待提示
			fmt.Println("  等待其他玩家行动...")
		}
	} else if c.GameInfo.GameStatus != "playing" {
		fmt.Println("  快捷键: [exit]退出")
	}
	fmt.Println("  其他输入: 发送聊天消息")
}

// getStatusName 获取状态名称
func getStatusName(status PlayerStatus) string {
	switch status {
	case StatusWaiting:
		return "等待中"
	case StatusPlaying:
		return "游戏中"
	case StatusChecked:
		return "已看牌"
	case StatusFolded:
		return "已弃牌"
	case StatusLost:
		return "已输掉"
	case StatusAllIn:
		return "全押"
	default:
		return "未知"
	}
}

// Connect 连接服务器
func (c *GameClient) Connect(addr string) error {
	// 解析地址，如果包含scheme则直接使用，否则添加ws://前缀
	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	if strings.Contains(addr, "://") {
		parsed, err := url.Parse(addr)
		if err != nil {
			return fmt.Errorf("解析地址失败: %w", err)
		}
		u = *parsed
		if u.Scheme == "http" {
			u.Scheme = "ws"
		} else if u.Scheme == "https" {
			u.Scheme = "wss"
		}
		if !strings.HasSuffix(u.Path, "/ws") {
			u.Path = "/ws"
		}
	}

	fmt.Printf("正在连接到服务器: %s\n", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("连接服务器失败: %w", err)
	}

	c.Conn = conn
	c.IsOnline = true

	fmt.Printf("已连接到服务器: %s\n", addr)
	return nil
}

// Send 发送消息
func (c *GameClient) Send(msg Message) error {
	if !c.IsOnline {
		return fmt.Errorf("客户端未连接")
	}

	err := c.Conn.WriteJSON(msg)
	if err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}

	return nil
}

// Receive 接收消息(在独立协程中运行)
func (c *GameClient) Receive() {
	for {
		var msg interface{}
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("连接意外断开: %v\n", err)
			}
			break
		}

		c.handleMessage(msg)
	}

	// 检查是否为玩家主动退出
	if !c.IsOnline {
		// 玩家主动退出，优雅关闭，不报错
		return
	} else {
		// 连接意外断开，提示用户并退出
		fmt.Println("\n=== 服务器连接已断开 ===")
		fmt.Println("连接已断开，程序将退出。")
		c.IsOnline = false
		c.Close()
		os.Exit(0)
	}
}

// handleMessage 处理接收到的消息
func (c *GameClient) handleMessage(msg interface{}) {
	// 更新最后收到消息的时间
	c.LastPing = time.Now()

	msgMap, ok := msg.(map[string]interface{})
	if !ok {
		fmt.Printf("未知消息类型: %v\n", msg)
		return
	}

	action, ok := msgMap["action"].(string)
	if !ok {
		fmt.Printf("消息缺少action字段: %v\n", msg)
		return
	}

	payload := msgMap["payload"]

	// fmt.Printf("[DEBUG] handleMessage: action=%s\n", action)

	switch action {
	case "response":
		if resp, ok := payload.(map[string]interface{}); ok {
			success, _ := resp["success"].(bool)
			message, _ := resp["message"].(string)
			var data interface{}
			if d, ok := resp["data"]; ok {
				data = d
			}

			// 检查是否为 leave 动作的响应
			if success && message == "已退出房间" {
				c.GameInfo = GameInfo{
					RoomID:       0,
					MasterID:     0,
					LastWinnerID: 0,
					Players:      []PlayerInfo{},
					CurrentBet:   0,
					Pot:          0,
					CurrentTurn:  0,
					GameStatus:   "",
				}
				c.IsExiting = false
				// 清空消息日志，显示全新的大厅视图
				c.MsgLog = make([]string, 0, MaxLogSize)
				c.AddLog("已回到大厅")
				c.DrawTable()
				return
			}

			c.handleResponse(Response{
				Success: success,
				Message: message,
				Data:    data,
			})
		}
	case "system":
		if broadcast, ok := payload.(map[string]interface{}); ok {
			msgType, _ := broadcast["type"].(string)
			content, _ := broadcast["content"].(string)
			var data interface{}
			if d, ok := broadcast["data"]; ok {
				data = d
			}
			c.handleBroadcast(BroadcastMessage{
				Type:    msgType,
				Content: content,
				Data:    data,
			})
		}
	default:
		fmt.Printf("未知的action类型: %s\n", action)
	}
}

// handleResponse 处理响应消息
func (c *GameClient) handleResponse(resp Response) {
	// 更新最后收到消息的时间
	c.LastPing = time.Now()

	// 标记是否已经调用过 DrawTable
	shouldDraw := true

	if resp.Success {
		// 跳过心跳响应，不显示在日志中
		if resp.Message == "pong" {
			return
		}

		c.AddLog(fmt.Sprintf("[成功] %s", resp.Message))
		if resp.Data != nil {
			if dataMap, ok := resp.Data.(map[string]interface{}); ok {
				// 尝试获取玩家ID（登录成功时返回）
				if id, ok := dataMap["id"].(float64); ok {
					c.PlayerID = int(id)
					// c.AddLog(fmt.Sprintf("[系统] 登录成功，ID: %d", c.PlayerID))
				}

				// 处理手牌数据（看牌时返回）
				if cardsData, ok := dataMap["cards"].([]interface{}); ok {
					c.MyCards = make([]Card, 0, len(cardsData))
					for _, cardInterface := range cardsData {
						if cardMap, ok := cardInterface.(map[string]interface{}); ok {
							// 转换花色 (从数字转换)
							var suit Suit
							if s, ok := cardMap["suit"].(float64); ok {
								suit = Suit(int(s))
							}

							// 转换点数 (从数字转换)
							var rank Rank
							if r, ok := cardMap["rank"].(float64); ok {
								rank = Rank(int(r))
							}

							c.MyCards = append(c.MyCards, Card{
								Suit:  suit,
								Rank:  rank,
								Value: int(rank),
							})
						}
					}
					// 看牌成功：更新玩家状态为已看牌（仅当确实收到手牌数据时）
					if c.MyPlayer != nil {
						c.MyPlayer.Status = StatusChecked
						c.HasChecked = true // 持久化标记已看牌
					}
				}
				if playerData, ok := dataMap["player"].(map[string]interface{}); ok {
					var playerID int
					if id, ok := playerData["id"].(float64); ok {
						playerID = int(id)
					}
					var name string
					if n, ok := playerData["name"].(string); ok {
						name = n
					}
					if name != "" {
						var chips int
						if c, ok := playerData["chips"].(float64); ok {
							chips = int(c)
						}
						var status PlayerStatus
						if s, ok := playerData["status"].(float64); ok {
							status = PlayerStatus(int(s))
						}
						// 只更新 MyPlayer 的信息，不要重新创建对象
						// 这样可以保持与 GameInfo.Players 中元素的引用关系
						if c.MyPlayer != nil {
							c.MyPlayer.ID = playerID
							c.MyPlayer.Name = name
							c.MyPlayer.Chips = chips
							// 如果已经看牌，保留已看牌状态
							if c.MyPlayer.Status != StatusChecked {
								c.MyPlayer.Status = status
							}
						} else {
							c.MyPlayer = &PlayerInfo{
								ID:     playerID,
								Name:   name,
								Chips:  chips,
								Status: status,
							}
						}
					}
				}
				if roomID, ok := dataMap["room_id"].(float64); ok {
					oldRoomID := c.GameInfo.RoomID
					newRoomID := int(roomID)
					c.GameInfo.RoomID = newRoomID

					// 如果退出房间（room_id为0），清理相关数据
					if c.GameInfo.RoomID == 0 {
						c.GameInfo.Players = []PlayerInfo{}
						c.GameInfo.Pot = 0
						c.GameInfo.CurrentBet = 0
						c.GameInfo.CurrentTurn = 0
						c.GameInfo.LastWinnerID = 0
						c.LastWinAmount = 0 // 重置上一次获胜金额
						c.MyCards = []Card{}
						if c.MyPlayer != nil {
							c.MyPlayer.Status = StatusWaiting
						}
						// 重置退出状态，允许在大厅中再次输入 exit 退出程序
						c.IsExiting = false
						// 清空消息日志，显示全新的大厅视图
						c.MsgLog = make([]string, 0, MaxLogSize)
						// 强制重绘，此时会自动显示大厅视图
						c.DrawTable()
						// 已经调用过 DrawTable，不再在函数末尾调用
						shouldDraw = false
					} else if oldRoomID == 0 && newRoomID > 0 {
						// 从大厅进入房间，清空消息日志
						c.MsgLog = make([]string, 0, MaxLogSize)
						c.LastWinAmount = 0 // 重置上一次获胜金额
						// 加入房间成功，重置退出状态
						c.IsExiting = false
					} else {
						// 房间内的其他操作
						c.IsExiting = false
					}
				}
				if rooms, ok := dataMap["rooms"].([]interface{}); ok {
					c.AddLog(fmt.Sprintf("[系统] 房间列表更新，共 %d 个房间", len(rooms)))
					if len(rooms) == 0 {
						c.AddLog("[系统] 当前暂无房间，您可以创建新房间")
					} else {
						c.AddLog("[系统] === 房间列表 ===")
						for _, r := range rooms {
							if room, ok := r.(map[string]interface{}); ok {
								roomID, _ := room["id"].(float64)
								masterName, _ := room["master_name"].(string)
								playerCount, _ := room["player_count"].(float64)
								status, _ := room["status"].(string)
								c.AddLog(fmt.Sprintf("[系统] 房间%d: 房主%s | 玩家%d | 状态%s",
									int(roomID), masterName, int(playerCount), status))
							}
						}
						c.AddLog("[系统] =================")
					}
				}
			}
		}
	} else {
		// 检查是否为退出失败消息
		if strings.Contains(resp.Message, "对局正在进行中") {
			c.AddLog("【禁止退出】当前正在对局中，请先弃牌或等待本局结束。")
		} else {
			c.AddLog(fmt.Sprintf("[错误] %s", resp.Message))
		}
	}

	// 只在需要时绘制界面
	if shouldDraw {
		c.DrawTable()
	}
}

// handleBroadcast 处理广播消息
func (c *GameClient) handleBroadcast(msg BroadcastMessage) {
	switch msg.Type {
	case "system":
		// 检查是否为比牌结果消息
		if strings.Contains(msg.Content, "发起比牌") && strings.Contains(msg.Content, "战败出局") {
			// 比牌结果用醒目的符号标注
			c.AddLog(fmt.Sprintf("[系统] %s", msg.Content))

			// 如果自己是败者，UI 立即切换为 [已出局] 状态
			if c.MyPlayer != nil && strings.Contains(msg.Content, c.MyPlayer.Name+" 战败出局") {
				c.MyPlayer.Status = StatusFolded
			}
		} else if strings.Contains(msg.Content, "房主") && strings.Contains(msg.Content, "退出") {
			// 房主退出消息
			c.AddLog(fmt.Sprintf("[系统] %s", msg.Content))

			// 从消息中提取新房主ID
			if data, ok := msg.Data.(map[string]interface{}); ok {
				if newMasterID, ok := data["new_master_id"].(float64); ok {
					c.GameInfo.MasterID = int(newMasterID)
					// 重新查找新房主在玩家列表中的信息
					for i := range c.GameInfo.Players {
						if c.GameInfo.Players[i].ID == c.GameInfo.MasterID {
							c.GameInfo.Players[i].Name = c.GameInfo.Players[i].Name + "(M)"
							break
						}
					}
				}
			}
		} else {
			c.AddLog(fmt.Sprintf("[系统] %s", msg.Content))
		}
	case "chat":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			playerName, _ := data["player_name"].(string)
			c.AddLog(fmt.Sprintf("[聊天] %s: %s", playerName, msg.Content))
		} else {
			c.AddLog(fmt.Sprintf("[聊天] %s", msg.Content))
		}
	case "game_update":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			if pot, ok := data["pot"].(float64); ok {
				c.GameInfo.Pot = int(pot)
			}
			if currentBet, ok := data["current_bet"].(float64); ok {
				c.GameInfo.CurrentBet = int(currentBet)
			}
			if gameState, ok := data["game_status"].(string); ok {
				c.GameInfo.GameStatus = gameState

				// fmt.Printf("[DEBUG] gameState: old=%s, new=%s, HasChecked=%v, len(MyCards)=%d\n", c.LastGameState, gameState, c.HasChecked, len(c.MyCards))
				if gameState == "playing" && c.LastGameState != "playing" {
					// 游戏刚开始，清空上一局残留的数据
					c.MyCards = []Card{}
					c.HasChecked = false
					if c.MyPlayer != nil {
						c.MyPlayer.Status = StatusWaiting
					}
				}
				c.LastGameState = gameState
			}
			if currentTurn, ok := data["current_turn"].(float64); ok {
				c.GameInfo.CurrentTurn = int(currentTurn)
			}
			if lastWinnerID, ok := data["last_winner_id"].(float64); ok {
				c.GameInfo.LastWinnerID = int(lastWinnerID)
				// 查找获胜者并显示胜利信息
				if lastWinnerID > 0 && c.GameInfo.GameStatus == "waiting" {
					var winAmount int
					if winAmountData, ok := data["last_win_amount"].(float64); ok {
						winAmount = int(winAmountData)
					}
					// 只有当获胜金额发生变化时才打印消息，防止重复打印
					if winAmount != c.LastWinAmount {
						for _, player := range c.GameInfo.Players {
							if player.ID == int(lastWinnerID) {
								c.AddLog(fmt.Sprintf("[系统] 游戏结束，%s 获胜，获得 %d 筹码", player.Name, winAmount))
								c.LastWinAmount = winAmount // 更新上一次获胜金额
								break
							}
						}
					}
				}
			}
			if masterID, ok := data["master_id"].(float64); ok {
				c.GameInfo.MasterID = int(masterID)
			}
			if ante, ok := data["ante"].(float64); ok {
				c.GameInfo.Ante = int(ante)
			}
			if currentSingleBet, ok := data["current_single_bet"].(float64); ok {
				c.GameInfo.CurrentSingleBet = int(currentSingleBet)
			}
			if players, ok := data["players"].([]interface{}); ok {
				c.GameInfo.Players = make([]PlayerInfo, 0, len(players))
				for _, p := range players {
					if playerMap, ok := p.(map[string]interface{}); ok {
						if id, ok := playerMap["id"].(float64); ok {
							if name, ok := playerMap["name"].(string); ok {
								if chips, ok := playerMap["chips"].(float64); ok {
									if roundBet, ok := playerMap["round_bet"].(float64); ok {
										if status, ok := playerMap["status"].(float64); ok {
											playerStatus := PlayerStatus(int(status))
											c.GameInfo.Players = append(c.GameInfo.Players, PlayerInfo{
												ID:       int(id),
												Name:     name,
												Chips:    int(chips),
												RoundBet: int(roundBet),
												Status:   playerStatus,
											})
											if c.PlayerID == int(id) {
												c.MyPlayer = &c.GameInfo.Players[len(c.GameInfo.Players)-1]
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		// 只显示有意义的消息，不显示纯状态更新
		if msg.Content != "" && msg.Content != "游戏状态更新" {
			c.AddLog(fmt.Sprintf("[游戏] %s", msg.Content))
		}
		// 如果消息内容为空，说明只是数据更新，不需要刷新界面
		// 这样可以避免在登录后重复绘制大厅
		if msg.Content == "" {
			return
		}
	default:
		c.AddLog(fmt.Sprintf("[广播] 未知类型: %s: %s", msg.Type, msg.Content))
	}
	c.DrawTable()
}

// Login 登录
func (c *GameClient) Login(name string) error {
	msg := NewMessage(ActionLogin, LoginPayload{Name: name})
	return c.Send(msg)
}
func (c *GameClient) Chat(content string) error {
	msg := NewMessage(ActionChat, ChatPayload{Content: content})
	return c.Send(msg)
}

// Bet 下注（闷注/加注）
func (c *GameClient) Bet(amount int) error {
	msg := NewMessage(ActionBet, BetPayload{Amount: amount})
	return c.Send(msg)
}

// Call 跟注
func (c *GameClient) Call() error {
	msg := NewMessage(ActionCall, nil)
	return c.Send(msg)
}

// Fold 弃牌
func (c *GameClient) Fold() error {
	msg := NewMessage(ActionFold, nil)
	return c.Send(msg)
}

// Check 看牌
func (c *GameClient) Check() error {
	msg := NewMessage(ActionCheck, nil)
	return c.Send(msg)
}

// AllIn 全押
func (c *GameClient) AllIn() error {
	msg := NewMessage(ActionAllIn, nil)
	return c.Send(msg)
}

// Leave 离开游戏
func (c *GameClient) Leave() error {
	msg := NewMessage(ActionLeave, nil)
	return c.Send(msg)
}

// Ping 发送心跳
func (c *GameClient) Ping() error {
	msg := NewMessage(ActionPing, nil)
	return c.Send(msg)
}

// Compare 比牌
func (c *GameClient) Compare(targetID int) error {
	msg := NewMessage(ActionCompare, ComparePayload{TargetID: targetID})
	return c.Send(msg)
}

// ListRooms 列出房间
func (c *GameClient) ListRooms() error {
	msg := NewMessage(ActionListRooms, nil)
	return c.Send(msg)
}

// CreateRoom 创建房间
func (c *GameClient) CreateRoom(name string) error {
	msg := NewMessage(ActionCreateRoom, CreateRoomPayload{Name: name})
	return c.Send(msg)
}

// JoinRoom 加入房间
func (c *GameClient) JoinRoom(roomID int) error {
	msg := NewMessage(ActionJoinRoom, JoinRoomPayload{RoomID: roomID})
	return c.Send(msg)
}

// StartGame 开始游戏
func (c *GameClient) StartGame() error {
	msg := NewMessage(ActionStartGame, nil)
	return c.Send(msg)
}

// Close 关闭连接
func (c *GameClient) Close() error {
	if c.Conn != nil {
		c.IsOnline = false
		return c.Conn.Close()
	}
	return nil
}

// HandleInput 处理用户输入
func (c *GameClient) HandleInput(input string) {
	// fmt.Printf("[DEBUG] HandleInput 收到输入: '%s'\n", input)

	input = strings.TrimSpace(input)
	// fmt.Printf("[DEBUG] TrimSpace 后: '%s'\n", input)

	if input == "" {
		// fmt.Println("[DEBUG] 输入为空，返回")
		return
	}

	// 大厅模式指令
	if c.GameInfo.RoomID == 0 {
		// fmt.Printf("[DEBUG] 大厅模式，调用 handleLobbyInput\n")
		c.handleLobbyInput(input)
		return
	}

	// fmt.Printf("[DEBUG] 房间模式，处理游戏指令\n")

	// 使用 Fields 自动处理所有空格，parts[0] 是命令，parts[1] 是参数
	parts := strings.Fields(input)
	// fmt.Printf("[DEBUG] Fields 解析结果: %v, 数量: %d\n", parts, len(parts))

	if len(parts) == 0 {
		// fmt.Println("[DEBUG] parts 为空，返回")
		return
	}

	command := strings.ToLower(parts[0])
	// fmt.Printf("[DEBUG] 解析命令: '%s', 参数数量: %d\n", command, len(parts))

	// 调试信息：打印当前回合信息
	if c.MyPlayer != nil {
		// fmt.Printf("[DEBUG] 当前回合玩家ID: %d, 我的玩家ID: %d\n", c.GameInfo.CurrentTurn, c.MyPlayer.ID)
	} else {
		// fmt.Println("[DEBUG] MyPlayer 为 nil")
	}
	// fmt.Printf("[DEBUG] RoomID: %d\n", c.GameInfo.RoomID)

	// 本地回合检查：非自己回合时输入游戏动作指令，本地提示并阻止发送
	if c.MyPlayer != nil && c.GameInfo.CurrentTurn != c.MyPlayer.ID {
		// fmt.Println("[DEBUG] 进入回合检查逻辑")
		switch command {
		case "k", "b", "c", "f", "r", "v":
			currentPlayerName := "未知"
			for _, player := range c.GameInfo.Players {
				if player.ID == c.GameInfo.CurrentTurn {
					currentPlayerName = player.Name
					break
				}
			}
			// fmt.Printf("[DEBUG] 回合检查失败，阻止发送命令: %s\n", command)
			c.AddLog(fmt.Sprintf("[提示] 还没轮到你呢，请等待 %s 行动", currentPlayerName))
			c.DrawTable()
			return
		}
	} else {
		// fmt.Println("[DEBUG] 回合检查通过，可以发送命令")
	}

	// fmt.Printf("[DEBUG] 准备进入 switch，command = '%s'\n", command)

	switch command {
	case "k":
		// 检查玩家是否已弃牌
		if c.MyPlayer != nil && (c.MyPlayer.Status == StatusFolded || c.MyPlayer.Status == StatusLost) {
			c.AddLog("[提示] 你已弃牌，无法进行操作")
			c.DrawTable()
			return
		}
		// fmt.Printf("[DEBUG] 进入看牌命令分支\n")
		// 调试信息：打印看牌前的状态
		if c.MyPlayer != nil {
			// fmt.Printf("[DEBUG] 看牌前: MyPlayer.Status=%d\n", c.MyPlayer.Status)
		}
		if err := c.Check(); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 看牌失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	case "v":
		// 检查玩家是否已弃牌
		if c.MyPlayer != nil && (c.MyPlayer.Status == StatusFolded || c.MyPlayer.Status == StatusLost) {
			c.AddLog("[提示] 你已弃牌，无法进行操作")
			c.DrawTable()
			return
		}
		// fmt.Printf("[DEBUG] 进入比牌命令分支\n")
		if len(parts) < 2 {
			c.AddLog("[错误] 比牌需要指定目标玩家ID，格式: v ID")
			c.DrawTable()
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			c.AddLog("[错误] ID必须是数字，例如: v 3")
			c.DrawTable()
			return
		}
		// fmt.Printf("[DEBUG] 准备调用 Compare，目标ID: %d\n", targetID)
		c.AddLog(fmt.Sprintf("正在向玩家 %d 发起比牌...", targetID))
		if err := c.Compare(targetID); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 比牌失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	case "c":
		// 检查玩家是否已弃牌
		if c.MyPlayer != nil && (c.MyPlayer.Status == StatusFolded || c.MyPlayer.Status == StatusLost) {
			c.AddLog("[提示] 你已弃牌，无法进行操作")
			c.DrawTable()
			return
		}
		// fmt.Printf("[DEBUG] 进入跟注命令分支\n")
		if err := c.Call(); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 跟注失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	case "b":
		// 检查玩家是否已弃牌
		if c.MyPlayer != nil && (c.MyPlayer.Status == StatusFolded || c.MyPlayer.Status == StatusLost) {
			c.AddLog("[提示] 你已弃牌，无法进行操作")
			c.DrawTable()
			return
		}
		// fmt.Printf("[DEBUG] 进入闷注命令分支\n")
		amount := c.GameInfo.CurrentBet
		if len(parts) > 1 {
			customAmount, err := strconv.Atoi(parts[1])
			if err == nil && customAmount > 0 {
				amount = customAmount
			}
		}
		if err := c.Bet(amount); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 闷注失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	case "r":
		// 检查玩家是否已弃牌
		if c.MyPlayer != nil && (c.MyPlayer.Status == StatusFolded || c.MyPlayer.Status == StatusLost) {
			c.AddLog("[提示] 你已弃牌，无法进行操作")
			c.DrawTable()
			return
		}
		// fmt.Printf("[DEBUG] 进入加注命令分支\n")
		// 调试信息：打印玩家状态
		if c.MyPlayer != nil {
			// fmt.Printf("[DEBUG] 加注检查: MyPlayer.Status=%d, StatusChecked=%d\n", c.MyPlayer.Status, StatusChecked)
		}
		if c.MyPlayer != nil && c.MyPlayer.Status != StatusChecked {
			c.AddLog("[错误] 未看牌玩家不能进行加注操作，请使用闷注(b)或跟注(c)")
			c.DrawTable()
			return
		}
		amount := c.GameInfo.CurrentBet * 2
		if len(parts) > 1 {
			customAmount, err := strconv.Atoi(parts[1])
			if err == nil && customAmount > 0 {
				amount = customAmount
			}
		}
		if err := c.Bet(amount); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 加注失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	case "f":
		// fmt.Printf("[DEBUG] 进入弃牌命令分支\n")
		if err := c.Fold(); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 弃牌失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	case "s":
		// fmt.Printf("[DEBUG] 进入开始游戏命令分支\n")
		if err := c.StartGame(); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 开始游戏失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	case "exit":
		// fmt.Printf("[DEBUG] 进入退出命令分支\n")
		if err := c.Leave(); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 退出房间失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播

	default:
		// fmt.Printf("[DEBUG] 进入 default 分支，未知命令: '%s'\n", command)
		c.AddLog(fmt.Sprintf("[聊天] %s", input))
		if err := c.Chat(input); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 发送聊天失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应/广播
	}
}

// handleLobbyInput 处理大厅模式输入
func (c *GameClient) handleLobbyInput(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "ls":
		if err := c.ListRooms(); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 列出房间失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应，handleResponse会调用DrawTable

	case "n":
		if err := c.CreateRoom(""); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 创建房间失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应，handleResponse会调用DrawTable

	case "exit":
		c.Close()
		fmt.Println("客户端已退出")
		os.Exit(0)

	case "j":
		if len(parts) < 2 {
			c.AddLog("[错误] 加入房间需要指定房间ID，格式: j ID")
			c.DrawTable()
		}
		roomID, err := strconv.Atoi(parts[1])
		if err != nil {
			c.AddLog("[错误] ID必须是数字，例如: j 1")
			c.DrawTable()
			return
		}
		if err := c.JoinRoom(roomID); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 加入房间失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应，handleResponse会调用DrawTable

	default:
		c.AddLog(fmt.Sprintf("[聊天] %s", input))
		if err := c.Chat(input); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 发送聊天失败: %v", err))
			c.DrawTable()
		}
		// 成功发送请求后，等待服务器响应，handleBroadcast会调用DrawTable
	}
}

// RunDashboard 仪表盘模式运行客户端
func (c *GameClient) RunDashboard() {
	// 启动接收消息的协程
	go c.Receive()

	// 记录最后收到消息的时间
	lastMsgTime := time.Now()

	// 启动心跳协程和超时检测
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if c.IsOnline {
				c.Ping()

				// 检查是否超过30秒没有收到服务器响应
				if time.Since(lastMsgTime) > 30*time.Second {
					fmt.Println("\n=== 服务器连接已断开 ===")
					fmt.Println("心跳超时，服务器连接已断开，程序将退出。")
					c.IsOnline = false
					c.Close()
					os.Exit(1)
				}
			}
		}
	}()

	// 监听消息接收，更新最后消息时间
	go func() {
		for {
			time.Sleep(1 * time.Second)
			// 如果有新消息，会更新c.LastPing
			if c.LastPing.After(lastMsgTime) {
				lastMsgTime = c.LastPing
			}
		}
	}()

	// 初始化最后消息时间
	lastMsgTime = time.Now()

	// 读取用户输入
	scanner := bufio.NewScanner(os.Stdin)

	// 创建超时通道
	timeoutChan := make(chan bool, 1)
	inputChan := make(chan string, 1)

	// 启动超时定时器（30秒后自动生成昵称）
	go func() {
		time.Sleep(30 * time.Second)
		if c.MyPlayer == nil {
			timeoutChan <- true
		}
	}()

	// 启动输入读取协程
	go func() {
		// fmt.Println("[DEBUG] 输入协程启动，等待首次输入...")
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			// fmt.Printf("[DEBUG] 输入协程读取到首次输入: '%s'\n", input)
			inputChan <- input
		}
		// fmt.Println("[DEBUG] 输入协程结束")
	}()

	// 等待输入或超时
	select {
	case input := <-inputChan:
		if input == "" {
			return
		}
		// 特殊命令处理
		if input == "exit" {
			if c.GameInfo.RoomID > 0 {
				// 如果在房间内，仅尝试离开房间，不关闭连接
				c.HandleInput("exit")
			} else {
				// 如果在大厅，才彻底退出程序
				c.Close()
				fmt.Println("已退出游戏大厅，程序关闭。")
				return
			}
		} else {
			// 如果玩家还未登录，自动将输入作为昵称进行登录
			if c.MyPlayer == nil {
				if err := c.Login(input); err != nil {
					c.AddLog(fmt.Sprintf("[错误] 登录失败: %v", err))
					c.DrawTable()
				}
				// 登录成功后，服务器会返回响应，handleResponse会调用DrawTable，所以这里不需要再次调用
			} else {
				// 处理其他输入
				c.HandleInput(input)
			}
		}
	case <-timeoutChan:
		// 超时后自动生成随机中文昵称
		randomName := generateRandomChineseName()
		fmt.Printf("\n检测到您长时间未输入，已自动为您生成昵称: %s\n", randomName)
		if err := c.Login(randomName); err != nil {
			c.AddLog(fmt.Sprintf("[错误] 登录失败: %v", err))
			c.DrawTable()
		}
		// 登录成功后，服务器会返回响应，handleResponse会调用DrawTable，所以这里不需要再次调用
	}

	// fmt.Println("[DEBUG] 进入主循环")

	// 主循环
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		// 统一处理 exit 命令
		if input == "exit" {
			// 如果正在等待退出响应，忽略新的 exit 命令
			if c.IsExiting {
				continue
			}
			if c.GameInfo.RoomID > 0 {
				// 场景 A：在房间内，仅退回到大厅
				c.AddLog("正在退出房间...")
				c.IsExiting = true
				c.GameInfo.RoomID = 0
				c.DrawTable()
				c.Leave()
				// 注意：这里不要调用 HandleInput(exit)，继续循环等待服务器回传大厅信息
				continue
			} else {
				// 场景 B：在大厅内，直接关闭客户端
				c.Close()
				fmt.Println("已退出游戏大厅，程序关闭。")
				return
			}
		}

		// 其他输入处理...

		// 处理其他输入
		// fmt.Printf("[DEBUG] 准备调用 HandleInput\n")
		c.HandleInput(input)
		// fmt.Printf("[DEBUG] HandleInput 调用完成\n")
	}

	if err := scanner.Err(); err != nil {
		c.AddLog(fmt.Sprintf("[错误] 读取输入错误: %v", err))
		c.DrawTable()
	}
}

// generateRandomChineseName 生成随机中文昵称
func generateRandomChineseName() string {
	surnames := []string{
		"李", "王", "张", "刘", "陈", "杨", "赵", "黄", "周", "吴",
		"徐", "孙", "胡", "朱", "高", "林", "何", "郭", "马", "罗",
	}

	firstNames := []string{
		"伟", "芳", "娜", "秀英", "敏", "静", "丽", "强", "磊", "军",
		"洋", "勇", "艳", "杰", "娟", "涛", "明", "超", "秀兰", "霞",
		"平", "刚", "桂英", "玉兰", "萍", "飞", "志强", "桂兰", "玉兰", "秀珍",
	}

	adjectives := []string{
		"快乐", "勇敢", "聪明", "幸运", "神秘", "温柔", "活泼", "冷静", "热情", "幽默",
		"潇洒", "豪爽", "机智", "沉稳", "阳光", "乐观", "自信", "坚强", "善良", "真诚",
	}

	rand.Seed(time.Now().UnixNano())

	surname := surnames[rand.Intn(len(surnames))]
	firstName := firstNames[rand.Intn(len(firstNames))]

	// 30%概率添加形容词
	if rand.Float32() < 0.3 {
		adjective := adjectives[rand.Intn(len(adjectives))]
		return surname + firstName + adjective
	}

	return surname + firstName
}
