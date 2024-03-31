package roundmgr

import (
	"context"

	"github.com/1ykyk/rebro"
	"github.com/1ykyk/rebro/gossip/internal/round"
)

type roundManager interface {
	Stop(context.Context) error
	StartRound(context.Context, rebro.QuorumCommitment) (*round.Round, error)
	StopRound(context.Context, rebro.QuorumCommitment) error
	GetRound(context.Context) *round.Round
}

type roundMngr struct {
	rounds   map[uint64]*round.Round
	roundsCh chan *roundReq

	closeCh, closedCh chan struct{}
}

func newRoundMngr() *roundMngr {
	rmgr := &roundMngr{
		rounds:   make(map[uint64]*round.Round),
		roundsCh: make(chan *roundReq, 4),
	}

	go rmgr.stateLoop()
	return rmgr
}

func (rmgr *roundMngr) Stop(ctx context.Context) error {

}

// func (rmgr *roundMngr) Start() error {
// 	go rmgr.stateLoop()
//
// }
//
// func (rmgr *roundMngr) Stop() error {
//
// }

func (rmgr *roundMngr) StartRound(ctx context.Context, qcomm rebro.QuorumCommitment) error {
	// TODO implement me
	panic("implement me")
}

func (rmgr *roundMngr) StopRound(ctx context.Context, qcomm rebro.QuorumCommitment) error {
	// TODO implement me
	panic("implement me")
}

func (rmgr *roundMngr) GetRound(ctx context.Context, qcomm rebro.MessageID) error {
	// TODO implement me
	panic("implement me")
}

func (rmgr *roundMngr) stateLoop() {
	for {
		select {
		case req := <-rmgr.roundsCh:
			switch req.tp {
			case start:
				// todo: notify an error back?
				_, ok := rmgr.rounds[req.qcomm.Round()]
				if ok {
					break
				}

				rmgr.rounds[req.qcomm.Round()] = req.qcomm

				// publish
			case stop:
				delete(rmgr.rounds, req.qcomm.Round())
			case get:
				qcom, ok := rmgr.rounds[req.qcomm.Round()]
				if ok {
					break
				}

			default:
				panic("invalid request type")
			}
		}
	}
}

type roundReqTp uint8

const (
	start roundReqTp = iota
	stop
	get
)

type roundReq struct {
	tp    roundReqTp
	qcomm rebro.QuorumCommitment
	resp  chan<- rebro.QuorumCommitment
}
