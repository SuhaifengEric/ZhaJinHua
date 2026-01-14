package main

import (
	"math/rand/v2"
)

// Deck 牌堆结构体
type Deck struct {
	cards []Card // 牌堆中的牌
}

// NewDeck 创建一个新的牌堆,包含52张牌
func NewDeck() *Deck {
	deck := &Deck{
		cards: make([]Card, 0, 52),
	}

	suits := []Suit{Spade, Heart, Diamond, Club}
	ranks := []Rank{Rank2, Rank3, Rank4, Rank5, Rank6, Rank7, Rank8, Rank9, Rank10, RankJack, RankQueen, RankKing, RankAce}

	for _, suit := range suits {
		for _, rank := range ranks {
			deck.cards = append(deck.cards, NewCard(suit, rank))
		}
	}

	return deck
}

// Shuffle 洗牌,使用 Fisher-Yates 算法
func (d *Deck) Shuffle() {
	n := len(d.cards)
	for i := n - 1; i > 0; i-- {
		j := rand.IntN(i + 1)
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	}
}

// Deal 发牌,返回指定数量的牌
func (d *Deck) Deal(count int) []Card {
	if count <= 0 {
		return []Card{}
	}

	if count > len(d.cards) {
		count = len(d.cards)
	}

	dealt := d.cards[:count]
	d.cards = d.cards[count:]

	return dealt
}

// Remaining 返回牌堆中剩余的牌数
func (d *Deck) Remaining() int {
	return len(d.cards)
}

// Reset 重置牌堆,重新生成52张牌
func (d *Deck) Reset() {
	*d = *NewDeck()
}

// Peek 查看牌堆顶部的牌(不移除)
func (d *Deck) Peek() (Card, bool) {
	if len(d.cards) == 0 {
		return Card{}, false
	}
	return d.cards[0], true
}

// String 返回牌堆的字符串表示
func (d *Deck) String() string {
	if len(d.cards) == 0 {
		return "牌堆: 空"
	}

	result := "牌堆:\n"
	for i, card := range d.cards {
		if i > 0 && i%13 == 0 {
			result += "\n"
		}
		result += card.String() + " "
	}
	return result
}

// DealToPlayers 给多个玩家发牌,每个玩家发指定数量的牌
// 采用轮流发牌的方式，确保牌的随机性和公平性
func (d *Deck) DealToPlayers(players []*Player, cardsPerPlayer int) {
	// 外层循环是每一轮发牌
	for i := 0; i < cardsPerPlayer; i++ {
		// 内层循环是给每个玩家发一张牌
		for _, player := range players {
			if card, ok := d.DealOneCard(); ok {
				player.AddCard(card)
			}
		}
	}
}

// DealOneCard 发一张牌
func (d *Deck) DealOneCard() (Card, bool) {
	if len(d.cards) == 0 {
		return Card{}, false
	}
	card := d.cards[0]
	d.cards = d.cards[1:]
	return card, true
}

// IsEmpty 检查牌堆是否为空
func (d *Deck) IsEmpty() bool {
	return len(d.cards) == 0
}

// GetCards 获取牌堆中的所有牌(副本)
func (d *Deck) GetCards() []Card {
	cards := make([]Card, len(d.cards))
	copy(cards, d.cards)
	return cards
}
