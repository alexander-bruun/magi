package models

// Notification represents a notification object
type Notification struct {
	Type    string      // Type of notification (e.g., "library_created", "library_updated", etc.)
	Payload interface{} // Data associated with the notification
}

// Listener defines the interface for objects that want to listen to notifications
type Listener interface {
	Notify(notification Notification)
}

// ListenerRegistry is a registry of listeners interested in notifications
var ListenerRegistry []Listener

// AddListener adds a listener to the registry
func AddListener(listener Listener) {
	ListenerRegistry = append(ListenerRegistry, listener)
}

// NotifyListeners notifies all registered listeners with a given notification
func NotifyListeners(notification Notification) {
	for _, listener := range ListenerRegistry {
		listener.Notify(notification)
	}
}
