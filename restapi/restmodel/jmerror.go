package restmodel


type JMError struct {
	What string
	HttpCode int
}

func (this *JMError)Error()  string {
	return this.What
}