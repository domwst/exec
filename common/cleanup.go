package common

type Cleanup struct {
	cleanupActions []func()
}

func (c *Cleanup) AddAction(action func()) {
	c.cleanupActions = append(c.cleanupActions, action)
}

func (c *Cleanup) Do() {
	for _, action := range c.cleanupActions {
		action()
	}
	c.Discard()
}

func (c *Cleanup) Discard() {
	c.cleanupActions = []func(){}
}
