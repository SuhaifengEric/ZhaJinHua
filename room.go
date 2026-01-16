package main

import (
	"fmt"
	"sync"
)

// GameState 游戏状态
type GameState int

const (
	StateWaiting  GameState = 0 // 等待玩家就坐
	StatePlaying GameState = 1 // 下注中
	StateSettling GameState = 2 // 比牌结算
	MaxRounds    int       = 20 // 最大游戏轮数
)

// Room 房间结构体
type Room struct {
	ID              int           // 房间ID
	MasterID        int           // 房主ID
	LastWinnerID    int           // 上局赢家ID
	LastWinAmount   int           // 上局获胜筹码
	Players         []*Player    // 玩家列表
	Deck            *Deck         // 牌堆
	Pot             int           // 当前奖池
	Ante            int           // 底注
	CurrentBet      int           // 当前单注金额（已废弃，保留用于兼容）
	CurrentSingleBet int          // 当前单注标准（以闷牌玩家为基准，1倍）
	HasActualBet    bool          // 是否有实际闷注（超过底注的下注）
	TurnIndex       int           // 当前操作者索引
	GameState       GameState     // 游戏状态
	RoundCount      int           // 当前游戏轮数
	mu              sync.RWMutex  // 读写锁
}

// NewRoom 创建新房间
func NewRoom(id int) *Room {
	return &Room{
		ID:               id,
		MasterID:         0,
		Players:          make([]*Player, 0, 6),
		Deck:             NewDeck(),
		Pot:              0,
		Ante:             10,
		CurrentBet:       10,
		CurrentSingleBet: 10,
		TurnIndex:        0,
		GameState:        StateWaiting,
		RoundCount:       0,
	}
}

// AddPlayer 添加玩家到房间
func (r *Room) AddPlayer(player *Player) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Players) >= 6 {
		return false
	}

	r.Players = append(r.Players, player)
	return true
}

// RemovePlayer 从房间移除玩家
func (r *Room) RemovePlayer(playerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, player := range r.Players {
		if player.ID == playerID {
			r.Players = append(r.Players[:i], r.Players[i+1:]...)
			break
		}
	}
}

// GetPlayer 获取玩家
func (r *Room) GetPlayer(playerID int) *Player {
	// fmt.Printf("[DEBUG] GetPlayer开始，查找玩家ID=%d\n", playerID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	// fmt.Printf("[DEBUG] GetPlayer: 已获取读锁，玩家列表长度=%d\n", len(r.Players))

	for _, player := range r.Players {
		if player.ID == playerID {
			// fmt.Printf("[DEBUG] GetPlayer: 找到玩家ID=%d\n", playerID)
			return player
		}
	}
	// fmt.Printf("[DEBUG] GetPlayer: 未找到玩家ID=%d\n", playerID)
	return nil
}

// GetPlayerCount 获取玩家数量
func (r *Room) GetPlayerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Players)
}

// StartGame 开始游戏
func (r *Room) StartGame() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Players) < 2 {
		return fmt.Errorf("玩家数量不足,至少需要2名玩家")
	}

	// 重置牌堆，确保每次游戏都使用完整的52张牌
	r.Deck.Reset()
	fmt.Printf("[DEBUG] 房间 %d 牌堆重置完成，剩余牌数: %d\n", r.ID, r.Deck.Remaining())
	// 洗牌
	r.Deck.Shuffle()

	// 清空玩家手牌
	for _, player := range r.Players {
		player.ClearCards()
	}

	// 扣除底注
	for _, player := range r.Players {
		if player.Chips < r.Ante {
			return fmt.Errorf("玩家 %s 筹码不足,无法支付底注", player.Name)
		}
		player.Chips -= r.Ante
		player.RoundBet = r.Ante
		r.Pot += r.Ante
	}

	// 发牌
	r.Deck.DealToPlayers(r.Players, 3)

	// [DEBUG] 检查是否有重复发牌的情况
	dealtCards := make(map[string]bool)
	for _, player := range r.Players {
		for _, card := range player.Cards {
			cardKey := fmt.Sprintf("%d_%d", card.Suit, card.Rank)
			if dealtCards[cardKey] {
				fmt.Printf("[ERROR] 发现重复的牌: %s，玩家: %s\n", card.String(), player.Name)
				// 打印所有玩家的手牌用于调试
				for _, p := range r.Players {
					fmt.Printf("[DEBUG] 玩家 %s 的手牌: ", p.Name)
					for _, c := range p.Cards {
						fmt.Printf("%s ", c.String())
					}
					fmt.Printf("\n")
				}
				return fmt.Errorf("发牌错误：出现重复的牌 %s", card.String())
			}
			dealtCards[cardKey] = true
		}
		fmt.Printf("[DEBUG] 玩家 %s 发到的牌: ", player.Name)
		for _, card := range player.Cards {
			fmt.Printf("%s ", card.String())
		}
		fmt.Printf("\n")
	}

	// 设置玩家状态为游戏中
	for _, player := range r.Players {
		player.Status = StatusPlaying
	}

	// 设置游戏状态
	r.GameState = StatePlaying
	r.CurrentBet = r.Ante
	r.CurrentSingleBet = r.Ante
	r.HasActualBet = false // 重置是否有实际闷注的标记
	r.RoundCount = 0

	// 庄家起手规则：首局由房主行动，后续局由上局赢家行动
	if r.LastWinnerID == 0 {
		// 首局：房主先行动
		for i, player := range r.Players {
			if player.ID == r.MasterID {
				r.TurnIndex = i
				break
			}
		}
	} else {
		// 后续局：上局赢家先行动
		winnerFound := false
		for i, player := range r.Players {
			if player.ID == r.LastWinnerID {
				r.TurnIndex = i
				winnerFound = true
				break
			}
		}

		// 如果赢家已退出，找到下一个活跃玩家
		if !winnerFound {
			// 从索引0开始查找第一个活跃玩家
			for i := 0; i < len(r.Players); i++ {
				if r.Players[i].Status != StatusFolded && r.Players[i].Status != StatusLost {
					r.TurnIndex = i
					break
				}
			}
		}
	}

	return nil
}

// HandleAction 处理玩家动作
func (r *Room) HandleAction(player *Player, action Action, payload interface{}) error {
	// fmt.Printf("[DEBUG] Room.HandleAction开始，玩家ID=%d，动作=%v\n", player.ID, action)
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.GameState != StatePlaying {
		// fmt.Printf("[DEBUG] Room.HandleAction: 游戏不在进行中\n")
		return fmt.Errorf("游戏不在进行中")
	}

	// 检查是否轮到该玩家
	if r.Players[r.TurnIndex].ID != player.ID {
		// fmt.Printf("[DEBUG] Room.HandleAction: 不是你的回合，当前回合玩家ID=%d\n", r.Players[r.TurnIndex].ID)
		return fmt.Errorf("不是你的回合")
	}

	// fmt.Printf("[DEBUG] Room.HandleAction: 执行动作 %v\n", action)
	switch action {
	case ActionBet:
		amountFloat, ok := payload.(float64)
		if !ok {
			return fmt.Errorf("无效的下注金额")
		}
		return r.handleBet(player, amountFloat)
	case ActionCall:
		return r.handleCall(player)
	case ActionFold:
		return r.handleFold(player)
	case ActionCheck:
		return r.handleCheck(player)
	case ActionAllIn:
		return r.handleAllIn(player)
	case ActionCompare:
		return r.handleCompare(player, payload)
	default:
		return fmt.Errorf("不支持的动作")
	}
}

// handleBet 处理下注
func (r *Room) handleBet(player *Player, amountFloat float64) error {
	amount := int(amountFloat)

	// 下注金额必须 >= 底注
	if amount < r.Ante {
		return fmt.Errorf("下注金额不能低于底注 %d", r.Ante)
	}

	// 检查筹码是否足够
	if amount > player.Chips {
		return fmt.Errorf("筹码不足，下注需要 %d 筹码", amount)
	}

	// 根据玩家状态计算新的基准单注
	// CurrentSingleBet 统一表示“已看牌玩家的跟注标准”
	newSingleBet := amount
	if player.Status != StatusChecked {
		// 闷牌玩家加注X，则新的基准单注为X * 2
		newSingleBet = amount * 2
	}
	// 已看牌玩家加注X，则新的基准单注为X

	// 验证新的基准单注必须 ≥ 旧的基准单注
	if newSingleBet < r.CurrentSingleBet {
		return fmt.Errorf("加注金额不足，新的基准单注 %d 必须 ≥ 旧的基准单注 %d", newSingleBet, r.CurrentSingleBet)
	}

	// 扣除筹码
	player.Chips -= amount
	player.RoundBet += amount
	r.Pot += amount

	// 更新基准单注
	r.CurrentSingleBet = newSingleBet
	r.CurrentBet = player.RoundBet

	// 成功下注后，切换到下一个回合
	r.NextTurn()
	return nil
}

// handleCall 处理跟注
func (r *Room) handleCall(player *Player) error {
	// 根据玩家状态计算跟注金额
	// CurrentSingleBet 统一表示“已看牌玩家的跟注标准”
	// 已看牌玩家：支付 CurrentSingleBet
	// 闷牌玩家：支付 CurrentSingleBet / 2
	callAmount := r.CurrentSingleBet
	if player.Status != StatusChecked {
		// 闷牌玩家支付一半
		callAmount = r.CurrentSingleBet / 2
	}

	// 检查筹码是否足够
	if callAmount > player.Chips {
		return fmt.Errorf("筹码不足，跟注需要 %d 筹码", callAmount)
	}

	// 扣除筹码
	player.Chips -= callAmount
	player.RoundBet += callAmount
	r.Pot += callAmount
	r.CurrentBet = player.RoundBet

	// 成功跟注后，切换到下一个回合
	r.NextTurn()
	return nil
}

// handleFold 处理弃牌
func (r *Room) handleFold(player *Player) error {
	player.Fold()

	// 注意：不需要将弃牌玩家的 RoundBet 添加到奖池
	// 因为每次下注/跟注时已经将金额添加到奖池了
	// 这里只需要检查是否只剩一人并结算

	// 检查是否只剩一人
	activeCount := 0
	for _, p := range r.Players {
		if p.Status != StatusFolded && p.Status != StatusLost {
			activeCount++
		}
	}

	// fmt.Printf("[DEBUG] room.handleCompare: 检查活跃玩家数量，activeCount=%d\n", activeCount)
	if activeCount <= 1 {
		// fmt.Printf("[DEBUG] room.handleCompare: 活跃玩家<=1，调用Settling\n")
		r.Settling()
	} else {
		// fmt.Printf("[DEBUG] room.handleCompare: 活跃玩家>1，调用NextTurn\n")
		r.NextTurn()
	}

	// fmt.Printf("[DEBUG] room.handleCompare: 函数执行完成，返回nil\n")
	return nil
}

// handleCheck 处理看牌
func (r *Room) handleCheck(player *Player) error {
	player.CheckCards()
	return nil
}

// handleAllIn 处理全押
func (r *Room) handleAllIn(player *Player) error {
	amount := player.Chips

	// 玩家全押时，只能下注其实际拥有的筹码
	player.Chips = 0
	player.RoundBet += amount
	r.Pot += amount
	r.CurrentBet = player.RoundBet
	player.Status = StatusAllIn

	// 成功全押后，切换到下一个回合
	r.NextTurn()
	return nil
}

// handleCompare 处理比牌
func (r *Room) handleCompare(player *Player, payload interface{}) error {
	// fmt.Printf("[DEBUG] room.handleCompare开始，玩家ID=%d\n", player.ID)
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		// fmt.Printf("[DEBUG] room.handleCompare: 无效的比牌载荷\n")
		return fmt.Errorf("无效的比牌载荷")
	}

	targetIDFloat, ok := payloadMap["target_id"].(float64)
	if !ok {
		// fmt.Printf("[DEBUG] room.handleCompare: 无效的目标玩家ID\n")
		return fmt.Errorf("无效的目标玩家ID")
	}

	targetID := int(targetIDFloat)
	// fmt.Printf("[DEBUG] room.handleCompare: 目标玩家ID=%d\n", targetID)
	// 直接遍历玩家列表，避免在持有写锁时调用GetPlayer（会导致死锁）
	var target *Player
	for _, p := range r.Players {
		if p.ID == targetID {
			target = p
			break
		}
	}
	if target == nil {
		// fmt.Printf("[DEBUG] room.handleCompare: 目标玩家不存在\n")
		return fmt.Errorf("目标玩家不存在")
	}
	// fmt.Printf("[DEBUG] room.handleCompare: 找到目标玩家，ID=%d, Name=%s, Status=%v\n", target.ID, target.Name, target.Status)

	if target.Status == StatusFolded || target.Status == StatusLost {
		// fmt.Printf("[DEBUG] room.handleCompare: 目标玩家已弃牌或已出局\n")
		return fmt.Errorf("目标玩家已弃牌或已出局")
	}

	// 比牌：严格按照 豹子 > 顺金 > 金花 > 顺子 > 对子 > 单张 的顺序判定
	// fmt.Printf("[DEBUG] room.handleCompare: 调用CompareHands比较手牌\n")
	result := CompareHands(player, target)
	// fmt.Printf("[DEBUG] room.handleCompare: CompareHands返回结果=%d\n", result)

	var loser *Player
	if result == 1 {
		// 玩家胜，目标输
		// fmt.Printf("[DEBUG] room.handleCompare: 玩家胜，目标输\n")
		loser = target
	} else if result == -1 {
		// 目标胜，玩家输
		// fmt.Printf("[DEBUG] room.handleCompare: 目标胜，玩家输\n")
		loser = player
	} else {
		// 平局，发起比牌的玩家(p1)为输
		// fmt.Printf("[DEBUG] room.handleCompare: 平局，发起比牌的玩家为输\n")
		loser = player
	}

	// 将输家的状态设为 StatusFolded
	// fmt.Printf("[DEBUG] room.handleCompare: 设置输家状态，输家ID=%d, Name=%s\n", loser.ID, loser.Name)
	loser.Status = StatusFolded
	// 注意：不需要再次添加 loser.RoundBet 到奖池，因为各玩家的下注已经全部累积到奖池中
	// fmt.Printf("[DEBUG] room.handleCompare: 输家RoundBet=%d，当前奖池=%d\n", loser.RoundBet, r.Pot)

	// 检查是否只剩一人
	activeCount := 0
	for _, p := range r.Players {
		if p.Status != StatusFolded && p.Status != StatusLost {
			activeCount++
		}
	}
	// fmt.Printf("[DEBUG] room.handleCompare: 活跃玩家数量=%d\n", activeCount)

	if activeCount <= 1 {
		r.Settling()
	} else {
		r.NextTurn()
	}

	return nil
}

// NextTurn 下一个回合
func (r *Room) NextTurn() {
	// 累加轮数
	r.RoundCount++

	// 检查是否达到最大轮数，强制比牌
	if r.RoundCount >= MaxRounds {
		r.forceCompare()
		return
	}

	// 查找下一个活跃玩家，跳过所有已弃牌或已输掉的玩家
	for i := 0; i < len(r.Players); i++ {
		r.TurnIndex = (r.TurnIndex + 1) % len(r.Players)
		nextPlayer := r.Players[r.TurnIndex]

		if nextPlayer.Status != StatusFolded && nextPlayer.Status != StatusLost {
			return
		}
	}

	// 如果所有玩家都已弃牌或输掉，触发结算
	r.Settling()
}

// forceCompare 强制比牌
func (r *Room) forceCompare() {
	// 找出当前操作者（基准玩家）
	currentPlayer := r.Players[r.TurnIndex]
	if currentPlayer.Status == StatusFolded || currentPlayer.Status == StatusLost {
		// 如果当前操作者已弃牌或已输，直接结算
		r.Settling()
		return
	}

	// 找出所有活跃玩家（不包括基准玩家）
	var opponents []*Player
	for _, player := range r.Players {
		if player.ID != currentPlayer.ID && player.Status != StatusFolded && player.Status != StatusLost {
			opponents = append(opponents, player)
		}
	}

	// 如果没有对手，直接结算
	if len(opponents) == 0 {
		r.Settling()
		return
	}

	// 以当前操作者为基准，依次与对手比牌
	winner := currentPlayer
	for _, opponent := range opponents {
		result := CompareHands(winner, opponent)
		if result == -1 {
			// 对手胜，基准玩家输
			currentPlayer.Status = StatusLost
			// 注意：不需要添加 RoundBet 到奖池，因为每次下注时已经添加了
			winner = opponent
		} else if result == 1 {
			// 基准玩家胜，对手输
			opponent.Status = StatusLost
			// 注意：不需要添加 RoundBet 到奖池，因为每次下注时已经添加了
		} else {
			// 平局，双方都输
			currentPlayer.Status = StatusLost
			opponent.Status = StatusLost
			// 注意：不需要添加 RoundBet 到奖池，因为每次下注时已经添加了
		}
	}

	// 结算
	r.Settling()
}

// Settling 结算
func (r *Room) Settling() {
	r.GameState = StateSettling

	// 找出赢家
	var winner *Player
	for _, player := range r.Players {
		if player.Status != StatusFolded && player.Status != StatusLost {
			winner = player
			break
		}
	}

	if winner != nil {
		winner.Chips += r.Pot
		winner.Status = StatusWaiting
		r.LastWinnerID = winner.ID
		r.LastWinAmount = r.Pot // 保存获胜筹码用于广播
		fmt.Printf("房间 %d 本轮结束,赢家: %s, 获胜筹码: %d\n", r.ID, winner.Name, r.Pot)
	} else {
		r.LastWinAmount = 0
	}

	// 重置奖池
	r.Pot = 0
	r.CurrentBet = 0
	r.CurrentSingleBet = 0

	// 重置所有玩家状态为等待中，清空手牌
	for _, player := range r.Players {
		player.ResetRound()
		
		// 检查是否输光筹码（或不足以支付底注），发放低保
		if player.Chips < r.Ante {
			player.Chips += 1000
			msg := fmt.Sprintf("%s玩家筹码不足底注，系统自动为他发放1000低保", player.Name)
			r.SubsidyEvents = append(r.SubsidyEvents, msg)
			fmt.Printf("房间 %d: %s\n", r.ID, msg)
		}
	}

	// 设置游戏状态为等待中，允许开始新游戏
	r.GameState = StateWaiting
}

// Reset 重置房间
func (r *Room) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Pot = 0
	r.CurrentBet = 0
	r.CurrentSingleBet = 0
	r.GameState = StateWaiting
	r.TurnIndex = 0

	for _, player := range r.Players {
		player.ResetRound()
	}
}

// GetStateName 获取游戏状态名称
func (gs GameState) String() string {
	switch gs {
	case StateWaiting:
		return "等待中"
	case StatePlaying:
		return "游戏中"
	case StateSettling:
		return "结算中"
	default:
		return "未知"
	}
}

// GetRoomInfo 获取房间信息
func (r *Room) GetRoomInfo() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	players := make([]PlayerInfo, 0, len(r.Players))
	for _, player := range r.Players {
		playerInfo := PlayerInfo{
			ID:       player.ID,
			Name:     player.Name,
			Chips:    player.Chips,
			RoundBet: player.RoundBet,
			Status:   player.Status,
			Cards:    []Card{}, // 默认不发送手牌
		}
		players = append(players, playerInfo)
	}

	currentTurn := 0
	if len(r.Players) > 0 && r.TurnIndex >= 0 && r.TurnIndex < len(r.Players) {
		currentTurn = r.Players[r.TurnIndex].ID
	}

	return map[string]interface{}{
		"room_id":            r.ID,
		"master_id":          r.MasterID,
		"last_winner_id":     r.LastWinnerID,
		"last_win_amount":    r.LastWinAmount,
		"ante":               r.Ante,
		"current_single_bet": r.CurrentSingleBet,
		"players":            players,
		"pot":                r.Pot,
		"current_bet":        r.CurrentBet,
		"current_turn":       currentTurn,
		"game_status":        r.getGameStateString(),
		"round_count":        r.RoundCount,
	}
}

func (r *Room) getGameStateString() string {
	switch r.GameState {
	case StateWaiting:
		return "waiting"
	case StatePlaying:
		return "playing"
	case StateSettling:
		return "settling"
	default:
		return "unknown"
	}
}
