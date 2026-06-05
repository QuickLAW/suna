package chat

import "time"

func (m *Model) EnqueueGuardConfirm(g *GuardConfirmView) {
	if g == nil {
		return
	}
	if m.PendingGuard != nil {
		m.GuardQueue = append(m.GuardQueue, g)
		return
	}
	m.PendingGuard = g
	m.GuardCursor = 1
	m.GuardScroll = 0
	m.Loading = false
	m.Phase = PhaseIdle
	m.PhaseStart = time.Time{}
}

func (m *Model) AdvanceGuardQueue() {
	if len(m.GuardQueue) == 0 {
		m.PendingGuard = nil
		m.GuardCursor = 0
		m.GuardScroll = 0
		return
	}
	m.PendingGuard = m.GuardQueue[0]
	m.GuardScroll = 0
	copy(m.GuardQueue, m.GuardQueue[1:])
	m.GuardQueue[len(m.GuardQueue)-1] = nil
	m.GuardQueue = m.GuardQueue[:len(m.GuardQueue)-1]
	m.GuardCursor = 1
	m.Loading = false
	m.Phase = PhaseIdle
	m.PhaseStart = time.Time{}
}

func (m *Model) ResumeToolPhase(now time.Time) {
	m.Loading = true
	m.Phase = PhaseTool
	m.PhaseStart = now
}
