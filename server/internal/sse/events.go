package sse

import "encoding/json"

// EmitCardEvent broadcasts a card event to the board, including the card's version.
func EmitCardEvent(hub *Hub, boardID, eventType string, card any, senderID string) {
	data, err := json.Marshal(card)
	if err != nil {
		return
	}
	hub.Broadcast <- &BoardEvent{
		BoardID:   boardID,
		EventType: eventType,
		Data:      data,
		SenderID:  senderID,
	}
}

// EmitCommentEvent broadcasts a comment event to the board.
func EmitCommentEvent(hub *Hub, boardID, eventType string, comment any, senderID string) {
	data, err := json.Marshal(comment)
	if err != nil {
		return
	}
	hub.Broadcast <- &BoardEvent{
		BoardID:   boardID,
		EventType: eventType,
		Data:      data,
		SenderID:  senderID,
	}
}
