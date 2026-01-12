package main

import "fmt"

// Suit 花色类型
type Suit int

const (
	Spade   Suit = iota // 黑桃 ♠
	Heart                // 红桃 ♥
	Diamond              // 方块 ♦
	Club                 // 梅花 ♣
)

// Rank 点数类型
type Rank int

const (
	Rank2 Rank = iota + 2
	Rank3
	Rank4
	Rank5
	Rank6
	Rank7
	Rank8
	Rank9
	Rank10
	RankJack
	RankQueen
	RankKing
	RankAce
)

// Card 扑克牌结构体
type Card struct {
	Suit  Suit  `json:"suit"`  // 花色
	Rank  Rank  `json:"rank"`  // 点数
	Value int   `json:"value"` // 数值,用于排序和比较
}

// String 返回卡牌的字符串表示
func (c Card) String() string {
	var suitSymbol string
	switch c.Suit {
	case Spade:
		suitSymbol = "♠"
	case Heart:
		suitSymbol = "♥"
	case Diamond:
		suitSymbol = "♦"
	case Club:
		suitSymbol = "♣"
	}

	var rankSymbol string
	switch c.Rank {
	case RankJack:
		rankSymbol = "J"
	case RankQueen:
		rankSymbol = "Q"
	case RankKing:
		rankSymbol = "K"
	case RankAce:
		rankSymbol = "A"
	default:
		rankSymbol = fmt.Sprintf("%d", c.Rank)
	}

	return fmt.Sprintf("%s%s", suitSymbol, rankSymbol)
}

// NewCard 创建一张新牌
func NewCard(suit Suit, rank Rank) Card {
	return Card{
		Suit:  suit,
		Rank:  rank,
		Value: int(rank),
	}
}

// GetSuitName 获取花色名称
func (s Suit) String() string {
	switch s {
	case Spade:
		return "黑桃"
	case Heart:
		return "红桃"
	case Diamond:
		return "方块"
	case Club:
		return "梅花"
	default:
		return "未知"
	}
}

// GetRankName 获取点数名称
func (r Rank) String() string {
	switch r {
	case RankJack:
		return "J"
	case RankQueen:
		return "Q"
	case RankKing:
		return "K"
	case RankAce:
		return "A"
	default:
		return fmt.Sprintf("%d", r)
	}
}
