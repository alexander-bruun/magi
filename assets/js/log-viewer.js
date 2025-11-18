/**
 * WebSocket Log Viewer
 * Reusable component for streaming logs via WebSocket
 */

class LogViewer {
    constructor(options) {
        this.wsUrl = options.wsUrl;
        this.containerId = options.containerId;
        this.outputId = options.outputId;
        this.maxEntries = options.maxEntries || 1000;
        this.reconnectInterval = options.reconnectInterval || 3000;
        this.maxReconnectAttempts = options.maxReconnectAttempts || Infinity;
        this.colorMap = options.colorMap || this.getDefaultColorMap();
        this.formatMessage = options.formatMessage || this.defaultFormatMessage.bind(this);
        
        this.ws = null;
        this.reconnectAttempts = 0;
        this.reconnectTimer = null;
        
        this.container = document.getElementById(this.containerId);
        this.output = document.getElementById(this.outputId);
        
        if (!this.container || !this.output) {
            console.error('LogViewer: Container or output element not found');
            return;
        }
        
        this.connect();
        this.setupCleanup();
    }
    
    getDefaultColorMap() {
        return {
            error: '#ff6b6b',
            stderr: '#ff6b6b',
            fatal: '#ff6b6b',
            warn: '#ffd93d',
            warning: '#ffd93d',
            success: '#6bcf7f',
            info: '#6bcf7f',
            default: '#6bcf7f'
        };
    }
    
    defaultFormatMessage(data) {
        return data.message;
    }
    
    connect() {
        try {
            this.ws = new WebSocket(this.wsUrl);
            
            this.ws.onopen = () => {
                console.log('LogViewer: WebSocket connected to', this.wsUrl);
                this.reconnectAttempts = 0;
                if (this.reconnectTimer) {
                    clearTimeout(this.reconnectTimer);
                    this.reconnectTimer = null;
                }
            };
            
            this.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.addLogEntry(data);
                } catch (e) {
                    console.error('LogViewer: Failed to parse message:', e);
                }
            };
            
            this.ws.onerror = (error) => {
                console.error('LogViewer: WebSocket error:', error);
            };
            
            this.ws.onclose = () => {
                console.log('LogViewer: WebSocket closed');
                this.handleReconnect();
            };
        } catch (e) {
            console.error('LogViewer: Failed to connect:', e);
            this.handleReconnect();
        }
    }
    
    handleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.log('LogViewer: Max reconnect attempts reached');
            return;
        }
        
        this.reconnectAttempts++;
        console.log(`LogViewer: Reconnecting... attempt ${this.reconnectAttempts}`);
        
        this.reconnectTimer = setTimeout(() => {
            this.connect();
        }, this.reconnectInterval);
    }
    
    addLogEntry(data) {
        const logEntry = document.createElement('div');
        logEntry.className = 'log-entry';
        
        // Determine color based on type or message content
        const color = this.getLogColor(data);
        logEntry.style.color = color;
        
        // Format and set the message
        logEntry.textContent = this.formatMessage(data);
        
        // Add to output
        this.output.appendChild(logEntry);
        
        // Limit entries for performance
        const entries = this.output.children;
        if (entries.length > this.maxEntries) {
            this.output.removeChild(entries[0]);
        }
        
        // Auto-scroll to bottom
        this.container.scrollTop = this.container.scrollHeight;
    }
    
    getLogColor(data) {
        const type = (data.type || '').toLowerCase();
        const message = (data.message || '').toLowerCase();
        
        // Check type first
        if (this.colorMap[type]) {
            return this.colorMap[type];
        }
        
        // Check message content for keywords
        for (const keyword in this.colorMap) {
            if (message.includes(keyword)) {
                return this.colorMap[keyword];
            }
        }
        
        return this.colorMap.default;
    }
    
    setupCleanup() {
        window.addEventListener('beforeunload', () => {
            this.disconnect();
        });
    }
    
    disconnect() {
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
        
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }
}

// Export for module systems or make globally available
if (typeof module !== 'undefined' && module.exports) {
    module.exports = LogViewer;
} else {
    window.LogViewer = LogViewer;
}
