package iox

import "io"

// `iox.Pipe` wraps the read and write ends of a pipe and provides helper
// methods to close them once.  Example:
//
//     pipe, err := iox.WrapPipe3(os.Pipe())
//     if err != nil {
//         return err
//     }
//     defer pipe.CloseBoth()
//     ...
//     // Tell consumer EOF.
//     if err := pipe.CloseW(); err != nil {
//         ...
//     }
//
type Pipe struct {
	R io.ReadCloser
	W io.WriteCloser
}

func WrapPipe(r io.ReadCloser, w io.WriteCloser) *Pipe {
	return &Pipe{R: r, W: w}
}

func WrapPipe3(r io.ReadCloser, w io.WriteCloser, err error) (*Pipe, error) {
	if err != nil {
		return nil, err
	}
	return &Pipe{R: r, W: w}, nil
}

func (p *Pipe) CloseBoth() error {
	err := p.CloseW()
	if err2 := p.CloseR(); err == nil {
		err = err2
	}
	return err
}

func (p *Pipe) CloseR() error {
	if p.R == nil {
		return nil
	}
	err := p.R.Close()
	p.R = nil
	return err
}

func (p *Pipe) CloseW() error {
	if p.W == nil {
		return nil
	}
	err := p.W.Close()
	p.W = nil
	return err
}
