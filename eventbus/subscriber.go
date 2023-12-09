package eventbus

type Subscriber interface {
	Subscript() []*Subscription
}
