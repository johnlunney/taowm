package main

import (
	"time"

	xp "github.com/BurntSushi/xgb/xproto"
)

func handleButtonPress(e xp.ButtonPressEvent) {
	s := screenContaining(e.RootX, e.RootY)
	button := e.Detail
	if e.State&xp.ModMaskControl != 0 {
		// Control-click is treated as a Middle Mouse Button.
		button = 2
	} else if e.State&xp.ModMask1 != 0 {
		// Alt-click is treated as a Right Mouse Button.
		button = 3
	}
	if button == 2 {
		// Middle Mouse Button is treated as Mouse-Wheel Up/Down.
		if e.State&xp.ModMaskShift != 0 {
			button = 4
		} else {
			button = 5
		}
	}
	k := s.workspace
	if k.listing != listNone && button > 3 {
		return
	}
	if k.listing == listWindows {
		button = 1
	} else if k.listing == listWorkspaces {
		button = 3
	}

	switch button {
	case 1:
		if k.index >= 0 {
			if iw, ok := k.list[k.index].(*window); ok {
				clickWindow(k.focusedFrame, iw)
			}
			k.listing, k.list, k.index = listNone, nil, -1
			k.configure()
			s.repaint()
		} else {
			doList(k, listWindows)
		}
	case 3:
		if k.index >= 0 {
			if ik, ok := k.list[k.index].(*workspace); ok {
				clickWorkspace(s, ik)
			}
			k.listing, k.list, k.index = listNone, nil, -1
			k.configure()
			s.repaint()
		} else {
			doList(k, listWorkspaces)
		}
	case 4:
		doWindow(k, prev)
	case 5:
		doWindow(k, next)
	}

	if button <= 3 {
		k = s.workspace
		w := k.focusedFrame.window
		if k.listing != listNone {
			w = nil
		}
		focus(w)
	}
}

func clickWindow(f0 *frame, w1 *window) {
	f1 := w1.frame
	w0 := f0.window
	if w0 == w1 {
		return
	}
	if w0 != nil {
		f0.window, w0.frame = nil, nil
	}
	if f1 != nil {
		f1.window, w1.frame = nil, nil
	}
	f0.window, w1.frame = w1, f0
	if w0 != nil && f1 != nil {
		f1.window, w0.frame = w0, f1
	}
}

func clickWorkspace(s0 *screen, k1 *workspace) {
	s1 := k1.screen
	k0 := s0.workspace
	if k0 == k1 {
		return
	}
	s0.workspace, k1.screen = k1, s0
	if s1 != nil {
		s1.workspace, k0.screen = k0, s1
	} else {
		k0.screen = nil
	}
	k0.layout()
	k1.layout()
	s0.repaint()
	s1.repaint()
}

func handleEnterNotify(e xp.EnterNotifyEvent) {
	w := findWindow(func(w *window) bool { return w.xWin == e.Event })
	if w == nil {
		return
	}
	k := w.workspace
	f0 := k.focusedFrame
	k.focusFrame(w.frame)
	if k.listing == listWindows && k.focusedFrame != f0 {
		k.makeList()
	}
}

func handleKeyPress(e xp.KeyPressEvent) {
	shift := 0
	if e.State&xp.ModMaskShift != 0 {
		shift = 1
	}
	keysym := int32(keysyms[e.Detail][shift])
	if shift != 0 {
		if keysym == 0 {
			keysym = int32(keysyms[e.Detail][0])
		}
		keysym = ^keysym
	}
	if a := actions[keysym]; a.do != nil {
		if a.do(screenContaining(e.RootX, e.RootY).workspace, a.arg) {
			pulseChan <- time.Now()
		}
	}
}

func handleMotionNotify(e xp.MotionNotifyEvent) {
	s := screenContaining(e.RootX, e.RootY)
	k := s.workspace
	f0 := k.focusedFrame
	if !k.fullscreen && k.listing != listWorkspaces {
		k.focusFrame(k.frameContaining(e.RootX, e.RootY))
	}
	if k.listing == listNone {
		return
	}
	i1, i0 := k.indexForPoint(e.RootX, e.RootY), k.index
	k.index = i1
	if k.listing == listWindows && k.focusedFrame != f0 {
		k.makeList()
		return
	}
	if i1 == i0 {
		return
	}

	x, y := clip(k)
	setForeground(colorPulseFocused)
	y += fontHeight + fontHeight1
	if i0 != -1 {
		drawText(x+fontWidth, y+int16(i0)*fontHeight, " ")
	}
	if i1 != -1 {
		drawText(x+fontWidth, y+int16(i1)*fontHeight, ">")
	}
	unclip()
}