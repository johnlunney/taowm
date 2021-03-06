package main

import (
	"fmt"
	"sync"
	"time"

	xp "github.com/BurntSushi/xgb/xproto"
)

var desktopColor uint32

func setForeground(c uint32) {
	if desktopColor == c {
		return
	}
	desktopColor = c
	check(xp.ChangeGCChecked(xConn, desktopXGC, xp.GcForeground, []uint32{c}))
}

func drawText(x, y int16, text string) {
	check(xp.ImageText8Checked(xConn, uint8(len(text)),
		xp.Drawable(desktopXWin), desktopXGC, x, y, text))
}

func clip(k *workspace) (int16, int16) {
	r := k.focusedFrame.rect
	if k.fullscreen || k.listing == listWorkspaces {
		r = k.mainFrame.rect
	}
	r.X, r.Y, r.Width, r.Height = r.X+2, r.Y+2, r.Width-3, r.Height-3
	check(xp.SetClipRectanglesChecked(
		xConn, xp.ClipOrderingUnsorted, desktopXGC, 0, 0, []xp.Rectangle{r}))
	return r.X, r.Y
}

func unclip() {
	r := xp.Rectangle{X: 0, Y: 0, Width: desktopWidth, Height: desktopHeight}
	check(xp.SetClipRectanglesChecked(
		xConn, xp.ClipOrderingUnsorted, desktopXGC, 0, 0, []xp.Rectangle{r}))
}

func handleExpose(e xp.ExposeEvent) {
	if e.Count != 0 {
		return
	}
	for _, s := range screens {
		k := s.workspace
		k.drawFrameBorders()
		if k.listing == listNone {
			continue
		}
		x, y := clip(k)
		y += int16(fontHeight1)
		setForeground(colorPulseUnfocused)
		drawText(x, y, time.Now().Format("2006-01-02  15:04  Monday"))
		y += int16(fontHeight)

		if k.listing == listWindows {
			setForeground(colorPulseFocused)
		}
		wNum := 0
		for i, item := range k.list {
			if iw, ok := item.(*window); ok {
				c0, c1 := ' ', ' '
				if k.listing == listWindows {
					if iw.frame == k.focusedFrame {
						c0 = '+'
					} else if iw.frame != nil {
						c0 = '-'
					} else if !iw.seen {
						c0 = '@'
					}
				}
				if iw.selected {
					c1 = '#'
				}
				drawText(x+int16(3*fontWidth), y+int16(i*fontHeight),
					fmt.Sprintf("%c%c %c %s", c0, c1, windowNames[wNum], iw.name))
				if wNum < len(windowNames)-1 {
					wNum++
				}
			} else {
				wNum = 0
			}
		}
		if k.listing == listWorkspaces {
			setForeground(colorPulseFocused)
			kNum := 0
			for i, item := range k.list {
				if ik, ok := item.(*workspace); ok {
					c := ' '
					if ik.screen == s {
						c = '+'
					} else if ik.screen != nil {
						c = '-'
					}
					drawText(x+int16(3*fontWidth), y+int16(i*fontHeight),
						fmt.Sprintf("%c  %s", c, workspaceNames[kNum]))
					if kNum < len(workspaceNames)-1 {
						kNum++
					}
				}
			}
		}
		if k.index >= 0 {
			drawText(x+int16(fontWidth), y+int16(k.index*fontHeight), ">")
		}
		unclip()
	}
}

var (
	pulseTimeLock sync.Mutex
	pulseTime     time.Time
)

var (
	pulseChan     = make(chan time.Time)
	pulseDoneChan = make(chan struct{})

	colorUnfocused uint32 = colorBaseUnfocused
	colorFocused   uint32 = colorBaseFocused
)

func init() {
	go runPulses()
}

func runPulses() {
	tChan := (<-chan time.Time)(nil)
	fChan := (chan func())(nil)
	for {
		select {
		case when := <-pulseChan:
			pulseTimeLock.Lock()
			pulseTime = when
			pulseTimeLock.Unlock()
			if tChan == nil {
				fChan = proactiveChan
			}
		case <-pulseDoneChan:
			if tChan == nil {
				tChan = time.After(pulseFrameDuration)
			}
		case <-tChan:
			tChan = nil
			fChan = proactiveChan
		case fChan <- pulse:
			fChan = nil
		}
	}
}

func pulse() {
	pulseTimeLock.Lock()
	t := pulseTime
	pulseTimeLock.Unlock()
	i := int(time.Since(t) * time.Duration(len(cos)) / pulseTotalDuration)
	if i < 0 {
		i = 0
	}
	anyUnseenWindows := findWindow(func(w *window) bool { return !w.seen }) != nil
	if !anyUnseenWindows && i > len(cos)/2 {
		i = len(cos) / 2
	}
	if quitting {
		colorFocused = colorQuitFocused
		colorUnfocused = colorQuitUnfocused
	} else {
		colorFocused = blend(colorPulseFocused, colorBaseFocused, uint32(i))
		colorUnfocused = blend(colorPulseUnfocused, colorBaseUnfocused, uint32(i))
	}
	for _, s := range screens {
		s.workspace.drawFrameBorders()
	}
	if i < len(cos)/2 || anyUnseenWindows {
		pulseDoneChan <- struct{}{}
	}
}

func blend(c0, c1, i uint32) uint32 {
	x := uint32(cos[i%uint32(len(cos))])
	y := 256 - x
	r0 := (c0 >> 16) & 0xff
	g0 := (c0 >> 8) & 0xff
	b0 := (c0 >> 0) & 0xff
	r1 := (c1 >> 16) & 0xff
	g1 := (c1 >> 8) & 0xff
	b1 := (c1 >> 0) & 0xff
	r2 := ((r0 * x) + (r1 * y)) / 256
	g2 := ((g0 * x) + (g1 * y)) / 256
	b2 := ((b0 * x) + (b1 * y)) / 256
	return r2<<16 | g2<<8 | b2
}

// cos was generated by:
//
//	const N = 32
//	for i := 0; i < N; i++ {
//		c := math.Cos(float64(i) * 2 * math.Pi / N)
//		fmt.Printf("%v,\n", int(0.5+(c+1)*256/2))
//	}
var cos = [32]uint16{
	256,
	254,
	246,
	234,
	219,
	199,
	177,
	153,
	128,
	103,
	79,
	57,
	37,
	22,
	10,
	2,
	0,
	2,
	10,
	22,
	37,
	57,
	79,
	103,
	128,
	153,
	177,
	199,
	219,
	234,
	246,
	254,
}

var windowNames = [...]byte{
	'1',
	'2',
	'3',
	'4',
	'5',
	'6',
	'7',
	'8',
	'9',
	'0',
	':',
}

var workspaceNames = [...][3]byte{
	{'F', '1', ' '},
	{'F', '2', ' '},
	{'F', '3', ' '},
	{'F', '4', ' '},
	{'F', '5', ' '},
	{'F', '6', ' '},
	{'F', '7', ' '},
	{'F', '8', ' '},
	{'F', '9', ' '},
	{'F', '1', '0'},
	{'F', '1', '1'},
	{'F', '1', '2'},
	{':', ':', ':'},
}
