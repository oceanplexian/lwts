/**
 * BoardStream — SSE client for real-time board updates.
 */
class BoardStream {
    constructor(boardId, handlers = {}) {
        this.boardId = boardId;
        this.handlers = handlers;
        this.es = null;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.maxReconnectAttempts = 10;
        this.reconnectAttempts = 0;
        this.reconnectTimer = null;
        this.intentionalClose = false;
    }

    connect() {
        this.intentionalClose = false;

        const token = Auth.getAccessToken() || '';
        const url = `/api/v1/boards/${this.boardId}/stream?token=${encodeURIComponent(token)}`;

        this.es = new EventSource(url);

        this.es.addEventListener('connected', (e) => {
            this.reconnectDelay = 1000;
            this.reconnectAttempts = 0;
            const data = JSON.parse(e.data);
            if (this.handlers.onConnected) this.handlers.onConnected(data);
        });

        this.es.addEventListener('card_created', (e) => {
            if (this.handlers.onCardCreated) this.handlers.onCardCreated(JSON.parse(e.data));
        });

        this.es.addEventListener('card_updated', (e) => {
            if (this.handlers.onCardUpdated) this.handlers.onCardUpdated(JSON.parse(e.data));
        });

        this.es.addEventListener('card_moved', (e) => {
            if (this.handlers.onCardMoved) this.handlers.onCardMoved(JSON.parse(e.data));
        });

        this.es.addEventListener('card_deleted', (e) => {
            if (this.handlers.onCardDeleted) this.handlers.onCardDeleted(JSON.parse(e.data));
        });

        this.es.addEventListener('cards_bulk_moved', (e) => {
            if (this.handlers.onCardsBulkMoved) this.handlers.onCardsBulkMoved(JSON.parse(e.data));
        });

        this.es.addEventListener('comment_added', (e) => {
            if (this.handlers.onCommentAdded) this.handlers.onCommentAdded(JSON.parse(e.data));
        });

        this.es.addEventListener('comment_deleted', (e) => {
            if (this.handlers.onCommentDeleted) this.handlers.onCommentDeleted(JSON.parse(e.data));
        });

        this.es.addEventListener('user_joined', (e) => {
            if (this.handlers.onUserJoined) this.handlers.onUserJoined(JSON.parse(e.data));
        });

        this.es.addEventListener('user_left', (e) => {
            if (this.handlers.onUserLeft) this.handlers.onUserLeft(JSON.parse(e.data));
        });

        this.es.addEventListener('heartbeat', () => {});

        this.es.onerror = () => {
            if (this.intentionalClose) return;
            this.es.close();
            if (this.handlers.onDisconnect) this.handlers.onDisconnect();
            this.scheduleReconnect();
        };
    }

    scheduleReconnect() {
        if (this.intentionalClose) return;

        this.reconnectAttempts++;
        if (this.reconnectAttempts > this.maxReconnectAttempts) {
            console.error(`BoardStream: gave up after ${this.maxReconnectAttempts} reconnect attempts`);
            return;
        }

        this.reconnectTimer = setTimeout(() => {
            this.connect();
        }, this.reconnectDelay);

        this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    }

    disconnect() {
        this.intentionalClose = true;
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
        if (this.es) {
            this.es.close();
            this.es = null;
        }
    }
}

window.BoardStream = BoardStream;
