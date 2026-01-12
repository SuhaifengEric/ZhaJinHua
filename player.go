package main

import "fmt"

// PlayerStatus 玩家状态类型
type PlayerStatus int

const (
	StatusWaiting    PlayerStatus = iota // 等待中
	StatusPlaying                         // 游戏中
	StatusFolded                          // 已弃牌
	StatusChecked                         // 已看牌
	StatusAllIn                           // 全押
	StatusLost                            // 已输掉
)

// Player 玩家结构体
type Player struct {
	ID        int           // 玩家唯一标识
	Name      string        // 玩家名称
	Cards     []Card        // 手牌
	Chips     int           // 筹码数量
	RoundBet  int           // 本轮下注额
	Status    PlayerStatus  // 当前状态
	HandInfo  *HandInfo     // 手牌分析结果缓存
	Room      *Room         // 所在房间
}

// NewPlayer 创建一个新玩家
func NewPlayer(id int, name string, chips int) *Player {
	return &Player{
		ID:       id,
		Name:     name,
		Cards:    make([]Card, 0, 3),
		Chips:    chips,
		RoundBet: 0,
		Status:   StatusWaiting,
	}
}

// AddCard 添加一张手牌
func (p *Player) AddCard(card Card) {
	p.Cards = append(p.Cards, card)
	p.HandInfo = nil // 清除缓存
}

// ClearCards 清空手牌
func (p *Player) ClearCards() {
	p.Cards = p.Cards[:0]
	p.HandInfo = nil // 清除缓存
}

// PlaceBet 下注
func (p *Player) PlaceBet(amount int) bool {
	if amount <= 0 {
		return false
	}
	if amount > p.Chips {
		return false
	}
	p.Chips -= amount
	p.RoundBet += amount
	return true
}

// Fold 弃牌
func (p *Player) Fold() {
	p.Status = StatusFolded
}

// CheckCards 看牌
func (p *Player) CheckCards() {
	if p.Status == StatusPlaying {
		p.Status = StatusChecked
	}
}

// AllIn 全押
func (p *Player) AllIn() {
	p.RoundBet += p.Chips
	p.Chips = 0
	p.Status = StatusAllIn
}

// ResetRound 重置玩家状态(新一轮)
func (p *Player) ResetRound() {
	p.ClearCards()
	p.RoundBet = 0
	p.Status = StatusWaiting
}

// SetPlaying 设置玩家为游戏中状态
func (p *Player) SetPlaying() {
	p.Status = StatusPlaying
}

// GetHandInfo 获取手牌分析结果(带缓存)
func (p *Player) GetHandInfo() HandInfo {
	if p.HandInfo == nil {
		info := AnalyzeHand(p.Cards)
		p.HandInfo = &info
		return info
	}
	return *p.HandInfo
}

// InvalidateHandCache 使手牌缓存失效
func (p *Player) InvalidateHandCache() {
	p.HandInfo = nil
}

// GetStatusName 获取状态名称
func (s PlayerStatus) String() string {
	switch s {
	case StatusWaiting:
		return "等待中"
	case StatusPlaying:
		return "游戏中"
	case StatusFolded:
		return "已弃牌"
	case StatusChecked:
		return "已看牌"
	case StatusAllIn:
		return "全押"
	case StatusLost:
		return "已输掉"
	default:
		return "未知状态"
	}
}

// String 返回玩家的字符串表示
func (p *Player) String() string {
	cardsStr := ""
	if len(p.Cards) > 0 {
		cardsStr = ", 手牌: "
		for i, card := range p.Cards {
			if i > 0 {
				cardsStr += " "
			}
			cardsStr += card.String()
		}
	}
	return fmt.Sprintf("玩家[%d] %s (筹码: %d, 本轮下注: %d, 状态: %s%s)",
		p.ID, p.Name, p.Chips, p.RoundBet, p.Status.String(), cardsStr)
}
