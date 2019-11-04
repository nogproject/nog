package nogfsodomd

type DomainLogicError struct {
	Reason string
}

func (err *DomainLogicError) Error() string {
	return "domain logic error: " + err.Reason
}

type GetentError struct {
	Reason string
}

func (err *GetentError) Error() string {
	return "getent error: " + err.Reason
}
