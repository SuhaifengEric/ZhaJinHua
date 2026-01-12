package main

// Action 消息动作类型
type Action string

const (
	ActionLogin     Action = "login"     // 登录
	ActionChat      Action = "chat"      // 聊天
	ActionBet       Action = "bet"       // 下注
	ActionCall      Action = "call"      // 跟注
	ActionFold      Action = "fold"      // 弃牌
	ActionCheck     Action = "check"     // 看牌
	ActionAllIn     Action = "allin"     // 全押
	ActionCompare   Action = "compare"   // 比牌
	ActionLeave     Action = "leave"     // 离开
	ActionPing      Action = "ping"      // 心跳
	ActionResponse  Action = "response"  // 响应
	ActionSystem    Action = "system"    // 系统
	ActionListRooms Action = "list_rooms" // 列出房间
	ActionCreateRoom Action = "create_room" // 创建房间
	ActionJoinRoom   Action = "join_room"   // 加入房间
	ActionStartGame Action = "start_game" // 开始游戏
)

// Message 消息结构体
type Message struct {
	Action  Action      `json:"action"`  // 动作类型
	Payload interface{} `json:"payload"` // 载荷数据
}

// LoginPayload 登录载荷
type LoginPayload struct {
	Name string `json:"name"` // 玩家名称
}

// ChatPayload 聊天载荷
type ChatPayload struct {
	Content string `json:"content"` // 聊天内容
}

// BetPayload 下注载荷
type BetPayload struct {
	Amount int `json:"amount"` // 下注金额
}

// ComparePayload 比牌载荷
type ComparePayload struct {
	TargetID int `json:"target_id"` // 目标玩家ID
}

// BroadcastMessage 广播消息结构体
type BroadcastMessage struct {
	Type    string      `json:"type"`    // 消息类型
	Content string      `json:"content"` // 消息内容
	Data    interface{} `json:"data"`    // 附加数据
}

// Response 响应结构体
type Response struct {
	Success bool        `json:"success"` // 是否成功
	Message string      `json:"message"` // 响应消息
	Data    interface{} `json:"data"`    // 响应数据
}

// PlayerInfo 玩家信息(用于网络传输)
type PlayerInfo struct {
	ID       int         `json:"id"`       // 玩家ID
	Name     string      `json:"name"`     // 玩家名称
	Chips    int         `json:"chips"`    // 筹码数量
	RoundBet int         `json:"round_bet"` // 本轮下注
	Status   PlayerStatus `json:"status"`   // 玩家状态
	Cards    []Card      `json:"cards"`    // 手牌(仅对自己可见)
}

// GameInfo 游戏信息(用于网络传输)
type GameInfo struct {
	RoomID          int          `json:"room_id"`          // 房间ID
	MasterID        int          `json:"master_id"`        // 房主ID
	LastWinnerID    int          `json:"last_winner_id"`   // 上局赢家ID
	Ante            int          `json:"ante"`             // 底注
	CurrentSingleBet int         `json:"current_single_bet"` // 当前单注标准
	Players         []PlayerInfo `json:"players"`          // 玩家列表
	CurrentBet      int          `json:"current_bet"`      // 当前下注额
	Pot             int          `json:"pot"`              // 奖池
	CurrentTurn     int          `json:"current_turn"`     // 当前回合玩家ID
	GameStatus      string       `json:"game_status"`      // 游戏状态
}

// RoomBrief 房间摘要(用于大厅列表展示)
type RoomBrief struct {
	ID        int    `json:"id"`         // 房间ID
	MasterID  int    `json:"master_id"`  // 房主ID
	MasterName string `json:"master_name"` // 房主名称
	PlayerCount int    `json:"player_count"` // 玩家数量
	Status     string `json:"status"`      // 房间状态
}

// CreateRoomPayload 创建房间载荷
type CreateRoomPayload struct {
	Name string `json:"name"` // 房间名称
}

// JoinRoomPayload 加入房间载荷
type JoinRoomPayload struct {
	RoomID int `json:"room_id"` // 房间ID
}

// NewMessage 创建新消息
func NewMessage(action Action, payload interface{}) Message {
	return Message{
		Action:  action,
		Payload: payload,
	}
}

// NewResponse 创建新响应
func NewResponse(success bool, message string, data interface{}) Response {
	return Response{
		Success: success,
		Message: message,
		Data:    data,
	}
}

// NewBroadcastMessage 创建新广播消息
func NewBroadcastMessage(msgType, content string, data interface{}) BroadcastMessage {
	return BroadcastMessage{
		Type:    msgType,
		Content: content,
		Data:    data,
	}
}
