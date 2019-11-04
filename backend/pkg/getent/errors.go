package getent

import "fmt"

type ExecError struct {
	What string
	Err  error
}

func (err *ExecError) Error() string {
	return fmt.Sprintf(
		"failed to execute %s: %v",
		err.What, err.Err,
	)
}

type ParseError struct {
	What string
	Text string
}

func (err *ParseError) Error() string {
	return fmt.Sprintf(
		"failed to parse %s '%s'",
		err.What, err.Text,
	)
}

type GroupConflictError struct {
	AGroup string
	AGid   uint32
	BGroup string
	BGid   uint32
}

func (err *GroupConflictError) Error() string {
	return fmt.Sprintf(
		"conflicting groups %d(%s) and %d(%s)",
		err.AGid, err.AGroup,
		err.BGid, err.BGroup,
	)
}
