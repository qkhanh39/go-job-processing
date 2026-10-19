package notification

type ChannelNotifier struct {
	ch chan struct{}
}

func NewChannelNotifier() *ChannelNotifier {
	return &ChannelNotifier{
		ch: make(chan struct{}, 1),
	}
}

func (n *ChannelNotifier) Notify() {
	select {
	case n.ch <- struct{}{}:
	default:
	}
}

func (n *ChannelNotifier) Channel() <-chan struct{} {
	return n.ch
}
