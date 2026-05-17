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

// EmitRaw broadcasts a pre-serialized JSON payload. Used by transport-layer
// helpers (board/card/comment handlers) so they don't bypass the hub's
// persistence pipeline.
func EmitRaw(hub *Hub, boardID, eventType string, data []byte, senderID string) {
	if hub == nil {
		return
	}
	hub.Broadcast <- &BoardEvent{
		BoardID:   boardID,
		EventType: eventType,
		Data:      data,
		SenderID:  senderID,
	}
}
