/**
 * Notification System
 * Handles fetching, displaying, and managing user notifications for new manga chapters
 */

(function() {
    'use strict';

    const POLL_INTERVAL = 30000; // Poll every 30 seconds
    let pollTimer = null;

    /**
     * Fetch unread notification count
     */
    async function fetchUnreadCount() {
        try {
            const response = await fetch('/api/notifications/unread-count', {
                credentials: 'include'
            });
            
            if (!response.ok) {
                if (response.status === 401) {
                    // User not authenticated, stop polling
                    stopPolling();
                    return 0;
                }
                throw new Error('Failed to fetch notification count');
            }
            
            const data = await response.json();
            return data.count;
        } catch (error) {
            console.error('Error fetching notification count:', error);
            return 0;
        }
    }

    /**
     * Update notification badge
     */
    function updateBadge(count) {
        const badge = document.getElementById('notification-badge');
        if (!badge) return;

        if (count > 0) {
            badge.textContent = count > 99 ? '99+' : count;
            badge.style.display = 'inline-flex';
        } else {
            badge.style.display = 'none';
        }
    }

    /**
     * Fetch notifications
     */
    async function fetchNotifications(unreadOnly = true) {
        try {
            const url = `/api/notifications?unread_only=${unreadOnly}`;
            const response = await fetch(url, {
                credentials: 'include'
            });
            
            if (!response.ok) {
                throw new Error('Failed to fetch notifications');
            }
            
            const data = await response.json();
            return data.notifications || [];
        } catch (error) {
            console.error('Error fetching notifications:', error);
            return [];
        }
    }

    /**
     * Format time ago
     */
    function timeAgo(dateString) {
        const date = new Date(dateString);
        const now = new Date();
        const seconds = Math.floor((now - date) / 1000);

        if (seconds < 60) return 'just now';
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
        if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`;
        return date.toLocaleDateString();
    }

    /**
     * Render notifications
     */
    function renderNotifications(notifications) {
        const list = document.getElementById('notification-list');
        const markAllBtn = document.getElementById('mark-all-read');
        
        if (!list) return;

        if (notifications.length === 0) {
            list.innerHTML = '<p class="uk-text-muted uk-text-center py-4">No new notifications</p>';
            if (markAllBtn) markAllBtn.style.display = 'none';
            return;
        }

        if (markAllBtn) markAllBtn.style.display = 'inline-block';

        const html = notifications.map(notif => `
            <div class="notification-item uk-card uk-card-small uk-card-body mb-2 ${notif.is_read ? 'read' : 'unread'}" data-id="${notif.id}">
                <div class="flex gap-2">
                    <div class="flex-1">
                        <a href="/series/${notif.media_slug}/${notif.chapter_slug}" 
                           hx-get="/series/${notif.media_slug}/${notif.chapter_slug}" 
                           hx-target="#content" 
                           hx-push-url="true"
                           class="notification-link"
                           onclick="markNotificationAsRead(${notif.id})">
                            <div class="font-semibold text-sm">${escapeHtml(notif.manga_name || 'Unknown Media')}</div>
                            <div class="text-sm uk-text-muted">${escapeHtml(notif.message)}</div>
                            <div class="text-xs uk-text-muted mt-1">${timeAgo(notif.created_at)}</div>
                        </a>
                    </div>
                    <div>
                        <button class="uk-btn uk-btn-small uk-btn-default" 
                                onclick="event.stopPropagation(); deleteNotification(${notif.id})"
                                title="Delete">
                            <uk-icon icon="X" ratio="0.8"></uk-icon>
                        </button>
                    </div>
                </div>
            </div>
        `).join('');

        list.innerHTML = html;
    }

    /**
     * Escape HTML to prevent XSS
     */
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * Mark notification as read
     */
    window.markNotificationAsRead = async function(notificationId) {
        try {
            const response = await fetch(`/api/notifications/${notificationId}/read`, {
                method: 'POST',
                credentials: 'include'
            });
            
            if (response.ok) {
                await refreshNotifications();
            }
        } catch (error) {
            console.error('Error marking notification as read:', error);
        }
    };

    /**
     * Delete notification
     */
    window.deleteNotification = async function(notificationId) {
        try {
            const response = await fetch(`/api/notifications/${notificationId}`, {
                method: 'DELETE',
                credentials: 'include'
            });
            
            if (response.ok) {
                await refreshNotifications();
            }
        } catch (error) {
            console.error('Error deleting notification:', error);
        }
    };

    /**
     * Mark all notifications as read
     */
    async function markAllAsRead() {
        try {
            const response = await fetch('/api/notifications/mark-all-read', {
                method: 'POST',
                credentials: 'include'
            });
            
            if (response.ok) {
                await refreshNotifications();
            }
        } catch (error) {
            console.error('Error marking all as read:', error);
        }
    }

    /**
     * Refresh notifications display
     */
    async function refreshNotifications() {
        const count = await fetchUnreadCount();
        updateBadge(count);
        
        const notifications = await fetchNotifications(true);
        renderNotifications(notifications);
    }

    /**
     * Load notifications when dropdown opens
     */
    function setupDropdownListener() {
        const bell = document.getElementById('notification-bell');
        if (!bell) return;

        bell.addEventListener('click', async function() {
            await refreshNotifications();
        });
    }

    /**
     * Setup mark all as read button
     */
    function setupMarkAllButton() {
        const markAllBtn = document.getElementById('mark-all-read');
        if (!markAllBtn) return;

        markAllBtn.addEventListener('click', async function(e) {
            e.preventDefault();
            await markAllAsRead();
        });
    }

    /**
     * Start polling for notifications
     */
    function startPolling() {
        // Initial fetch
        refreshNotifications();
        
        // Poll every 30 seconds
        pollTimer = setInterval(async () => {
            const count = await fetchUnreadCount();
            updateBadge(count);
        }, POLL_INTERVAL);
    }

    /**
     * Stop polling
     */
    function stopPolling() {
        if (pollTimer) {
            clearInterval(pollTimer);
            pollTimer = null;
        }
    }

    /**
     * Initialize notification system
     */
    function init() {
        // Check if notification bell exists (user is authenticated)
        const bell = document.getElementById('notification-bell');
        if (!bell) {
            return; // Not authenticated, don't initialize
        }

        setupDropdownListener();
        setupMarkAllButton();
        startPolling();

        // Cleanup on page unload
        window.addEventListener('beforeunload', stopPolling);
    }

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    // Re-initialize after HTMX swaps (in case navbar gets swapped)
    document.addEventListener('htmx:afterSwap', function(event) {
        if (event.detail.target && (event.detail.target.id === 'content' || event.detail.target.tagName === 'BODY')) {
            // Give the DOM a moment to settle
            setTimeout(init, 100);
        }
    });

})();
