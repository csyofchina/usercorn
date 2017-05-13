package trace

import (
	"encoding/binary"
	"github.com/pkg/errors"
	"io"

	"github.com/lunixbochs/usercorn/go/models"
)

var order = binary.LittleEndian

const (
	OP_NOP       = 0
	OP_FRAME     = 1
	OP_KEYFRAME  = 2
	OP_JMP       = 3
	OP_STEP      = 4
	OP_REG       = 5
	OP_SPREG     = 6
	OP_MEM_READ  = 7
	OP_MEM_WRITE = 8
	OP_MEM_MAP   = 9
	OP_MEM_UNMAP = 10
	OP_SYSCALL   = 11
	OP_EXIT      = 12
)

// used by frame, keyframe, and syscall
func packOps(w io.Writer, ops []models.Op) (total int, err error) {
	for _, v := range ops {
		if n, err := v.Pack(w); err != nil {
			return total + n, errors.Wrap(err, "packing op list")
		} else {
			total += n
		}
	}
	return total, nil
}

// used by frame, keyframe, and syscall
func unpackOps(r io.Reader, count int) (ops []models.Op, total int, err error) {
	ops = make([]models.Op, count)
	for i := 0; i < count; i++ {
		op, n, err := Unpack(r)
		if err != nil {
			return ops, total + n, errors.Wrap(err, "unpacking op list")
		} else {
			total += n
		}
		ops[i] = op
	}
	return ops, total, nil
}

func Unpack(r io.Reader) (models.Op, int, error) {
	var tmp [1]byte
	if _, err := r.Read(tmp[:]); err != nil {
		return nil, 0, err
	}
	var op models.Op
	switch tmp[0] {
	case OP_NOP:
		op = &OpNop{}
	case OP_JMP:
		op = &OpJmp{}
	case OP_STEP:
		op = &OpStep{}
	case OP_REG:
		op = &OpReg{}
	case OP_SPREG:
		op = &OpSpReg{}
	case OP_MEM_READ:
		op = &OpMemRead{}
	case OP_MEM_WRITE:
		op = &OpMemWrite{}
	case OP_MEM_MAP:
		op = &OpMemMap{}
	case OP_MEM_UNMAP:
		op = &OpMemUnmap{}
	case OP_SYSCALL:
		op = &OpSyscall{}
	case OP_FRAME:
		op = &OpFrame{}
	case OP_KEYFRAME:
		op = &OpKeyframe{}
	case OP_EXIT:
		op = &OpExit{}
	default:
		return nil, 0, errors.Errorf("Unknown op: %d", tmp[0])
	}
	n, err := op.Unpack(r)
	return op, n + 1, err
}

type OpNop struct{}

func (o *OpNop) Pack(w io.Writer) (int, error)   { return w.Write([]byte{OP_NOP}) }
func (o *OpNop) Unpack(r io.Reader) (int, error) { return 0, nil }

type OpExit struct{ OpNop }

func (o *OpExit) Pack(w io.Writer) (int, error) { return w.Write([]byte{OP_EXIT}) }

type OpJmp struct {
	Addr uint64
	Size uint32
}

func (o *OpJmp) Pack(w io.Writer) (int, error) {
	var tmp [1 + 8 + 4]byte
	tmp[0] = OP_JMP
	order.PutUint64(tmp[1:], o.Addr)
	order.PutUint32(tmp[9:], o.Size)
	return w.Write(tmp[:])
}

func (o *OpJmp) Unpack(r io.Reader) (int, error) {
	var tmp [8 + 4]byte
	n, err := io.ReadFull(r, tmp[:])
	if err == nil {
		o.Addr = order.Uint64(tmp[:])
		o.Size = order.Uint32(tmp[8:])
	}
	return n, err
}

type OpStep struct {
	Size uint8
}

func (o *OpStep) Pack(w io.Writer) (int, error) {
	return w.Write([]byte{OP_STEP, o.Size})
}

func (o *OpStep) Unpack(r io.Reader) (int, error) {
	var tmp [1]byte
	n, err := r.Read(tmp[:])
	if err == nil {
		o.Size = uint8(tmp[0])
	}
	return n, err
}

type OpReg struct {
	Num uint16
	Val uint64
}

func (o *OpReg) Pack(w io.Writer) (int, error) {
	var tmp [1 + 2 + 8]byte
	tmp[0] = OP_REG
	order.PutUint16(tmp[1:], o.Num)
	order.PutUint64(tmp[3:], o.Val)
	return w.Write(tmp[:])
}

func (o *OpReg) Unpack(r io.Reader) (int, error) {
	var tmp [2 + 8]byte
	n, err := io.ReadFull(r, tmp[:])
	if err == nil {
		o.Num = order.Uint16(tmp[:])
		o.Val = order.Uint64(tmp[2:])
	}
	return n, err
}

type OpSpReg struct {
	Num uint16
	Val []byte
}

func (o *OpSpReg) Pack(w io.Writer) (int, error) {
	var tmp [1 + 2 + 2]byte
	tmp[0] = OP_SPREG
	order.PutUint16(tmp[1:], o.Num)
	order.PutUint16(tmp[3:], uint16(len(o.Val)))
	total, err := w.Write(tmp[:])
	if err != nil {
		return total, err
	}
	n, err := w.Write(o.Val)
	return total + n, err
}

func (o *OpSpReg) Unpack(r io.Reader) (int, error) {
	var tmp [2 + 2]byte
	total, err := io.ReadFull(r, tmp[:])
	if err == nil {
		o.Num = order.Uint16(tmp[:])
		size := order.Uint16(tmp[2:])
		o.Val = make([]byte, size)
		n, err := io.ReadFull(r, o.Val)
		return total + n, err
	}
	return total, err
}

type OpMemRead struct {
	Addr uint64
	Data []byte
}

func (o *OpMemRead) Pack(w io.Writer) (int, error) {
	var tmp [1 + 8 + 4]byte
	tmp[0] = OP_MEM_READ
	order.PutUint64(tmp[1:], o.Addr)
	order.PutUint32(tmp[9:], uint32(len(o.Data)))
	total, err := w.Write(tmp[:])
	if err != nil {
		return total, err
	}
	n, err := w.Write(o.Data)
	return total + n, err
}

func (o *OpMemRead) Unpack(r io.Reader) (int, error) {
	return (*OpMemWrite)(o).Unpack(r)
}

type OpMemWrite struct {
	Addr uint64
	Data []byte
}

func (o *OpMemWrite) Pack(w io.Writer) (int, error) {
	var tmp [1 + 8 + 4]byte
	tmp[0] = OP_MEM_WRITE
	order.PutUint64(tmp[1:], o.Addr)
	order.PutUint32(tmp[9:], uint32(len(o.Data)))
	total, err := w.Write(tmp[:])
	if err != nil {
		return total, err
	}
	n, err := w.Write(o.Data)
	return total + n, err
}

func (o *OpMemWrite) Unpack(r io.Reader) (int, error) {
	var tmp [8 + 4]byte
	total, err := io.ReadFull(r, tmp[:])
	if err == nil {
		o.Addr = order.Uint64(tmp[:])
		size := order.Uint32(tmp[8:])
		o.Data = make([]byte, size)
		if n, err := io.ReadFull(r, o.Data); err != nil {
			return total + n, err
		} else {
			total += n
		}
	}
	return total, err
}

type OpMemMap struct {
	Addr uint64
	Size uint64
	Prot uint8
	Zero uint8
}

func (o *OpMemMap) Pack(w io.Writer) (int, error) {
	var tmp [1 + 8 + 8 + 1 + 1]byte
	tmp[0] = OP_MEM_MAP
	order.PutUint64(tmp[1:], o.Addr)
	order.PutUint64(tmp[9:], o.Size)
	tmp[17] = o.Prot
	tmp[18] = o.Zero
	return w.Write(tmp[:])
}

func (o *OpMemMap) Unpack(r io.Reader) (int, error) {
	var tmp [8 + 8 + 1 + 1]byte
	n, err := io.ReadFull(r, tmp[:])
	if err == nil {
		o.Addr = order.Uint64(tmp[:])
		o.Size = order.Uint64(tmp[8:])
		o.Prot = tmp[16]
		o.Zero = tmp[17]
	}
	return n, err
}

type OpMemUnmap struct {
	Addr uint64
	Size uint64
}

func (o *OpMemUnmap) Pack(w io.Writer) (int, error) {
	var tmp [1 + 8 + 8]byte
	tmp[0] = OP_MEM_UNMAP
	order.PutUint64(tmp[1:], o.Addr)
	order.PutUint64(tmp[9:], o.Size)
	return w.Write(tmp[:])
}

func (o *OpMemUnmap) Unpack(r io.Reader) (int, error) {
	var tmp [8 + 8]byte
	n, err := io.ReadFull(r, tmp[:])
	if err == nil {
		o.Addr = order.Uint64(tmp[:])
		o.Size = order.Uint64(tmp[8:])
	}
	return n, err
}

type OpSyscall struct {
	Num  uint32
	Ret  uint64
	Args []uint64
	Ops  []models.Op
}

func (o *OpSyscall) Pack(w io.Writer) (total int, err error) {
	// pack header
	var tmp [1 + 4 + 8 + 2 + 2]byte
	tmp[0] = OP_SYSCALL
	order.PutUint32(tmp[1:], o.Num)
	order.PutUint64(tmp[5:], o.Ret)
	order.PutUint16(tmp[13:], uint16(len(o.Args)))
	order.PutUint16(tmp[15:], uint16(len(o.Ops)))
	if total, err = w.Write(tmp[:]); err != nil {
		return total, err
	}

	// pack args
	tmp2 := make([]byte, len(o.Args)*8)
	for i, v := range o.Args {
		order.PutUint64(tmp2[i*8:], v)
	}
	if n, err := w.Write(tmp2); err != nil {
		return total + n, err
	} else {
		total += n
	}

	// pack sub-ops
	n, err := packOps(w, o.Ops)
	return total + n, err
}

func (o *OpSyscall) Unpack(r io.Reader) (int, error) {
	var tmp [4 + 8 + 2 + 2]byte
	total, err := io.ReadFull(r, tmp[:])
	if err == nil {
		// unpack header
		o.Num = order.Uint32(tmp[:])
		o.Ret = order.Uint64(tmp[4:])
		args := int(order.Uint16(tmp[12:]))
		count := int(order.Uint16(tmp[14:]))

		// unpack args
		tmp2 := make([]byte, 8*args)
		n, err := io.ReadFull(r, tmp2[:])
		if err != nil {
			return total + n, errors.Wrap(err, "syscall unpack")
		} else {
			total += n
		}
		o.Args = make([]uint64, args)
		for i := 0; i < args; i++ {
			o.Args[i] = order.Uint64(tmp2[i*8:])
		}

		// unpack sub-ops
		o.Ops, n, err = unpackOps(r, count)
		total += n
	}
	return total, errors.Wrap(err, "syscall unpack")
}

type OpKeyframe struct {
	Pid uint64
	Ops []models.Op
}

func (o *OpKeyframe) Pack(w io.Writer) (int, error) {
	// pack header
	var tmp [1 + 8 + 4]byte
	tmp[0] = OP_KEYFRAME
	order.PutUint64(tmp[1:], o.Pid)
	order.PutUint32(tmp[9:], uint32(len(o.Ops)))
	total, err := w.Write(tmp[:])
	if err != nil {
		return total, err
	}
	n, err := packOps(w, o.Ops)
	return total + n, err
}

func (o *OpKeyframe) Unpack(r io.Reader) (int, error) {
	return (*OpFrame)(o).Unpack(r)
}

type OpFrame struct {
	Pid uint64
	Ops []models.Op
}

func (o *OpFrame) Pack(w io.Writer) (int, error) {
	// pack header
	var tmp [1 + 8 + 4]byte
	tmp[0] = OP_FRAME
	order.PutUint64(tmp[1:], o.Pid)
	order.PutUint32(tmp[9:], uint32(len(o.Ops)))
	total, err := w.Write(tmp[:])
	if err != nil {
		return total, err
	}
	// pack sub-ops
	n, err := packOps(w, o.Ops)
	return total + n, err
}

func (o *OpFrame) Unpack(r io.Reader) (int, error) {
	var tmp [8 + 4]byte
	total, err := io.ReadFull(r, tmp[:])
	if err != nil {
		return total, errors.Wrap(err, "frame unpack")
	} else {
		o.Pid = order.Uint64(tmp[:])
		count := int(order.Uint32(tmp[8:]))
		// unpack sub-ops
		ops, n, err := unpackOps(r, count)
		o.Ops = ops
		return total + n, err
	}
}