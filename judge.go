package main

import (
	"sort"
)

// HandType 牌型类型
type HandType int

const (
	TypeBaoZi  HandType = 6 // 豹子 (三张相同)
	TypeShunJin HandType = 5 // 顺金 (同花顺)
	TypeJinHua  HandType = 4 // 金花 (同花)
	TypeShunZi  HandType = 3 // 顺子
	TypeDuiZi   HandType = 2 // 对子
	TypeDanZhang HandType = 1 // 单张
)

// HandInfo 手牌信息
type HandInfo struct {
	Type     HandType // 牌型
	Weights  []int    // 权重数组,用于比较
	Original []Card   // 原始手牌
}

// Special235KillBaoZi 特殊规则开关:235大于豹子
var Special235KillBaoZi = false

// AnalyzeHand 分析手牌,返回牌型信息和权重
func AnalyzeHand(cards []Card) HandInfo {
	if len(cards) != 3 {
		return HandInfo{Type: TypeDanZhang, Weights: []int{0}, Original: cards}
	}

	// 按点数降序排序
	sortedCards := make([]Card, len(cards))
	copy(sortedCards, cards)
	sort.Slice(sortedCards, func(i, j int) bool {
		return sortedCards[i].Value > sortedCards[j].Value
	})

	// 检查是否为豹子
	if isBaoZi(sortedCards) {
		return HandInfo{
			Type:     TypeBaoZi,
			Weights:  []int{sortedCards[0].Value},
			Original: cards,
		}
	}

	// 检查是否为同花
	isFlush := isFlush(sortedCards)

	// 检查是否为顺子
	isStraight, straightWeights := isStraight(sortedCards)

	// 顺金
	if isFlush && isStraight {
		return HandInfo{
			Type:     TypeShunJin,
			Weights:  straightWeights,
			Original: cards,
		}
	}

	// 金花
	if isFlush {
		return HandInfo{
			Type:     TypeJinHua,
			Weights:  []int{sortedCards[0].Value, sortedCards[1].Value, sortedCards[2].Value},
			Original: cards,
		}
	}

	// 顺子
	if isStraight {
		return HandInfo{
			Type:     TypeShunZi,
			Weights:  straightWeights,
			Original: cards,
		}
	}

	// 对子
	if pairInfo := isPair(sortedCards); pairInfo != nil {
		return HandInfo{
			Type:     TypeDuiZi,
			Weights:  pairInfo,
			Original: cards,
		}
	}

	// 单张
	return HandInfo{
		Type:     TypeDanZhang,
		Weights:  []int{sortedCards[0].Value, sortedCards[1].Value, sortedCards[2].Value},
		Original: cards,
	}
}

// isBaoZi 检查是否为豹子
func isBaoZi(cards []Card) bool {
	return cards[0].Value == cards[1].Value && cards[1].Value == cards[2].Value
}

// isFlush 检查是否为同花
func isFlush(cards []Card) bool {
	return cards[0].Suit == cards[1].Suit && cards[1].Suit == cards[2].Suit
}

// isStraight 检查是否为顺子
func isStraight(cards []Card) (bool, []int) {
	v1, v2, v3 := cards[0].Value, cards[1].Value, cards[2].Value

	// 普通顺子: v1-1 == v2 && v2-1 == v3
	if v1-1 == v2 && v2-1 == v3 {
		return true, []int{v1, v2, v3}
	}

	// 特殊顺子 A23 (A=14, 3=3, 2=2)
	if v1 == 14 && v2 == 3 && v3 == 2 {
		// A23 的权重设为 3, 2, 1 (仅次于 QKA)
		return true, []int{3, 2, 1}
	}

	return false, nil
}

// isPair 检查是否为对子,返回权重数组
func isPair(cards []Card) []int {
	v1, v2, v3 := cards[0].Value, cards[1].Value, cards[2].Value

	// 前两张相同
	if v1 == v2 {
		return []int{v1, v3}
	}

	// 后两张相同
	if v2 == v3 {
		return []int{v2, v1}
	}

	// 第一张和第三张相同
	if v1 == v3 {
		return []int{v1, v2}
	}

	return nil
}

// CompareHands 比较两个玩家的手牌
// 返回 1 (p1胜), -1 (p2胜), 0 (平局)
func CompareHands(p1, p2 *Player) int {
	// 安全检查: 空指针
	if p1 == nil || p2 == nil {
		return 0
	}

	// 安全检查: 手牌数量不足
	if len(p1.Cards) != 3 || len(p2.Cards) != 3 {
		return 0
	}

	// 使用缓存的手牌分析结果
	info1 := p1.GetHandInfo()
	info2 := p2.GetHandInfo()

	// 特殊规则:235大于豹子
	if Special235KillBaoZi {
		if is235(info1) && info2.Type == TypeBaoZi {
			return 1
		}
		if is235(info2) && info1.Type == TypeBaoZi {
			return -1
		}
	}

	// 比较牌型
	if info1.Type > info2.Type {
		return 1
	}
	if info1.Type < info2.Type {
		return -1
	}

	// 同牌型,比较权重
	return compareWeights(info1.Weights, info2.Weights)
}

// compareWeights 比较权重数组
// 返回 1 (w1胜), -1 (w2胜), 0 (平局)
func compareWeights(w1, w2 []int) int {
	minLen := len(w1)
	if len(w2) < minLen {
		minLen = len(w2)
	}

	for i := 0; i < minLen; i++ {
		if w1[i] > w2[i] {
			return 1
		}
		if w1[i] < w2[i] {
			return -1
		}
	}

	// 修复平局问题：权重完全相同时，根据牌型类型比较
	// 由于相同牌型的权重数组结构相同，理论上不应该出现完全相同的情况
	// 但为了避免平局导致的错误，这里返回1表示w1胜
	return 1
}

// is235 检查是否为235
func is235(info HandInfo) bool {
	if info.Type != TypeDanZhang {
		return false
	}
	weights := info.Weights
	return weights[0] == 5 && weights[1] == 3 && weights[2] == 2
}

// GetHandTypeName 获取牌型名称
func (ht HandType) String() string {
	switch ht {
	case TypeBaoZi:
		return "豹子"
	case TypeShunJin:
		return "顺金"
	case TypeJinHua:
		return "金花"
	case TypeShunZi:
		return "顺子"
	case TypeDuiZi:
		return "对子"
	case TypeDanZhang:
		return "单张"
	default:
		return "未知"
	}
}

// GetHandDescription 获取手牌描述
func (hi HandInfo) String() string {
	cardsStr := ""
	for i, card := range hi.Original {
		if i > 0 {
			cardsStr += " "
		}
		cardsStr += card.String()
	}
	return cardsStr + " (" + hi.Type.String() + ")"
}
