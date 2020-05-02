package main

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"os"
	"path/filepath"
	
	"github.com/go-gl/glfw/v3.3/glfw"
	lua "github.com/yuin/gopher-lua"
)

func luaRegister(l *lua.LState, name string, f func(*lua.LState) int) {
	l.Register(name, f)
}
func strArg(l *lua.LState, argi int) string {
	if !lua.LVCanConvToString(l.Get(argi)) {
		l.RaiseError("%v番目の引数が文字列ではありません。", argi)
	}
	return l.ToString(argi)
}
func numArg(l *lua.LState, argi int) float64 {
	num, ok := l.Get(argi).(lua.LNumber)
	if !ok {
		l.RaiseError("%v番目の引数が数ではありません。", argi)
	}
	return float64(num)
}
func boolArg(l *lua.LState, argi int) bool {
	return l.ToBool(argi)
}
func tableArg(l *lua.LState, argi int) *lua.LTable {
	return l.ToTable(argi)
}
func newUserData(l *lua.LState, value interface{}) *lua.LUserData {
	ud := l.NewUserData()
	ud.Value = value
	return ud
}
func toUserData(l *lua.LState, argi int) interface{} {
	if ud := l.ToUserData(argi); ud != nil {
		return ud.Value
	}
	return nil
}
func userDataError(l *lua.LState, argi int, udtype interface{}) {
	l.RaiseError("%v番目の引数が%Tではありません。", argi, udtype)
}

type InputDialog interface {
	Popup(title string) (ok bool)
	IsDone() bool
	GetStr() string
}

func newInputDialog() InputDialog {
	return newCommandLineInput()
}

type commandLineInput struct {
	str  string
	done bool
}

func newCommandLineInput() *commandLineInput {
	return &commandLineInput{done: true}
}
func (cli *commandLineInput) Popup(title string) bool {
	if !cli.done {
		return false
	}
	cli.done = false
	print(title + ": ")
	return true
}
func (cli *commandLineInput) IsDone() bool {
	if !cli.done {
		select {
		case cli.str = <-sys.commandLine:
			cli.done = true
		default:
		}
	}
	return cli.done
}
func (cli *commandLineInput) GetStr() string {
	if !cli.IsDone() {
		return ""
	}
	return cli.str
}

// Script Common

func scriptCommonInit(l *lua.LState) {
	// A bind to GO's SetGCPercent. A negative percentage disables garbage collection.
	luaRegister(l, "SetGCPercent", func(*lua.LState) int {
		debug.SetGCPercent(int(numArg(l, 1)))
		return 0
	})
	//----------------------------------------------------------------
	// Returns sys.externalShaderNames as a table.
	// It contains the list of filenames of shader loaded (Without extension)
	luaRegister(l, "GetShaderNames", func(*lua.LState) int {
		nameTable := l.NewTable()
		for _, name := range sys.externalShaderNames {
			nameTable.Append(lua.LString(name))
		}
		l.Push(nameTable)
		return 1
	})
	
	//----------------------------------------------------------------
	// Generate table listing all files in a given directory
	luaRegister(l, "GetDirectoryFiles", func(*lua.LState) int {
		dir := l.NewTable()
		filepath.Walk(strArg(l, 1), func(path string, info os.FileInfo, err error) error {
			dir.Append(lua.LString(path))
			return nil
		})
		l.Push(dir)
		return 1
	})
	
	//----------------------------------------------------------------
	luaRegister(l, "getFrameCount", func(l *lua.LState) int {
		l.Push(lua.LNumber(sys.frameCounter))
		return 1
	})
	
	luaRegister(l, "clearColor", func(l *lua.LState) int {
		a := int32(255)
		if l.GetTop() >= 4 {
			a = int32(numArg(l, 4))
		}
		col := uint32(int32(numArg(l, 3))&0xff | int32(numArg(l, 2))&0xff<<8 |
		int32(numArg(l, 1))&0xff<<16)
		FillRect(sys.scrrect, col, a)
		return 0
	})
	luaRegister(l, "fillRect", func(l *lua.LState) int {
		x1 := float32(numArg(l, 1))
		y1 := float32(numArg(l, 2))
		x2 := float32(numArg(l, 3))
		y2 := float32(numArg(l, 4))
		var ws, hs float32 = 1, 1
		if l.GetTop() >= 10 && boolArg(l, 10) { //auto scaling
			if l.GetTop() >= 11 && boolArg(l, 11) { //use screenpack localcoord
				ws = float32(sys.scrrect[2]) / MinF(float32(sys.luaLocalcoord[0]), float32(sys.gameWidth))
				hs = float32(sys.scrrect[3]) / MinF(float32(sys.luaLocalcoord[1]), float32(sys.gameHeight))
			} else {
				ws = float32(sys.scrrect[2]) / float32(sys.gameWidth)
				hs = float32(sys.scrrect[3]) / float32(sys.gameHeight)
				x1 += float32(sys.gameWidth-320)/2
				y1 += float32(sys.gameHeight-240)
			}
		}
		col := uint32(int32(numArg(l, 7))&0xff | int32(numArg(l, 6))&0xff<<8 | int32(numArg(l, 5))&0xff<<16)
		a := int32(int32(numArg(l, 8))&0xff | int32(numArg(l, 9))&0xff<<10)
		FillRect([4]int32{int32(x1*ws),int32(y1*hs),int32(x2*ws),int32(y2*hs)}, col, a)
		return 0
	})
	luaRegister(l, "fadeScreen", func(l *lua.LState) int {
		frame := float64(sys.frameCounter - int32(numArg(l, 2)))
		length := float64(numArg(l, 3))
		if frame > length || length <= 0 {
			l.Push(lua.LBool(false))
			return 1
		}
		r, g, b, a := int32(0), int32(0), int32(0), float64(0)
		if strArg(l, 1) == "fadeout" {
			a = math.Floor(float64(255) / length * frame)
		} else if strArg(l, 1) == "fadein" {
			a = math.Floor(255 - 255 * (frame - 1) / length)
		}
		if a < 0 {
			a = 0
		} else if a > 255 {
			a = 255
		}
		if l.GetTop() >= 6 {
			r = int32(numArg(l, 4))
			g = int32(numArg(l, 5))
			b = int32(numArg(l, 6))
		}
		col := uint32(int32(b)&0xff | int32(g)&0xff<<8 | int32(r)&0xff<<16)
		FillRect(sys.scrrect, col, int32(a))
		l.Push(lua.LBool(true))
		return 1
	})
	luaRegister(l, "sffNew", func(l *lua.LState) int {
		sff, err := loadSff(strArg(l, 1), false)
		if err != nil {
			l.RaiseError(err.Error())
		}
		l.Push(newUserData(l, sff))
		return 1
	})
	luaRegister(l, "sndNew", func(l *lua.LState) int {
		snd, err := LoadSnd(strArg(l, 1))
		if err != nil {
			l.RaiseError(err.Error())
		}
		l.Push(newUserData(l, snd))
		return 1
	})
	luaRegister(l, "fontNew", func(l *lua.LState) int {
		var height int32 = -1
		if l.GetTop() >= 2 {
			height = int32(numArg(l, 2))
		}
		fnt, err := loadFnt(strArg(l, 1), height)
		if err != nil {
			l.RaiseError(err.Error())
		}
		l.Push(newUserData(l, fnt))
		return 1
	})
	luaRegister(l, "fontGetDef", func(l *lua.LState) int {
		fnt, ok := toUserData(l, 1).(*Fnt)
		if !ok {
			userDataError(l, 1, fnt)
		}
		tbl := l.NewTable()
		tbl.RawSetString("Type", lua.LString(fnt.Type))
		subt := l.NewTable()
		subt.Append(lua.LNumber(fnt.Size[0]))
		subt.Append(lua.LNumber(fnt.Size[1]))
		tbl.RawSetString("Size", subt)
		subt = l.NewTable()
		subt.Append(lua.LNumber(fnt.Spacing[0]))
		subt.Append(lua.LNumber(fnt.Spacing[1]))
		tbl.RawSetString("Spacing", subt)
		subt = l.NewTable()
		subt.Append(lua.LNumber(fnt.offset[0]))
		subt.Append(lua.LNumber(fnt.offset[1]))
		tbl.RawSetString("offset", subt)
		l.Push(tbl)
		return 1
	})
	luaRegister(l, "fontGetTextWidth", func(*lua.LState) int {
		fnt, ok := toUserData(l, 1).(*Fnt)
		if !ok {
			userDataError(l, 1, fnt)
		}
		l.Push(lua.LNumber(fnt.TextWidth(strArg(l, 2))))
		return 1
	})
	luaRegister(l, "commandNew", func(l *lua.LState) int {
		l.Push(newUserData(l, NewCommandList(NewCommandBuffer())))
		return 1
	})
	luaRegister(l, "commandAdd", func(l *lua.LState) int {
		cl, ok := toUserData(l, 1).(*CommandList)
		if !ok {
			userDataError(l, 1, cl)
		}
		c, err := ReadCommand(strArg(l, 2), strArg(l, 3), NewCommandKeyRemap())
		if err != nil {
			l.RaiseError(err.Error())
		}
		cl.Add(*c)
		return 0
	})
	luaRegister(l, "commandGetState", func(l *lua.LState) int {
		cl, ok := toUserData(l, 1).(*CommandList)
		if !ok {
			userDataError(l, 1, cl)
		}
		l.Push(lua.LBool(cl.GetState(strArg(l, 2))))
		return 1
	})
	luaRegister(l, "commandInput", func(l *lua.LState) int {
		cl, ok := toUserData(l, 1).(*CommandList)
		if !ok {
			userDataError(l, 1, cl)
		}
		if cl.Input(int(numArg(l, 2))-1, 1, 0) {
			cl.Step(1, false, false, 0)
		}
		return 0
	})
	luaRegister(l, "commandBufReset", func(l *lua.LState) int {
		cl, ok := toUserData(l, 1).(*CommandList)
		if !ok {
			userDataError(l, 1, cl)
		}
		cl.BufReset()
		return 0
	})
	luaRegister(l, "inputDialogNew", func(l *lua.LState) int {
		l.Push(newUserData(l, newInputDialog()))
		return 1
	})
	luaRegister(l, "inputDialogPopup", func(l *lua.LState) int {
		id, ok := toUserData(l, 1).(InputDialog)
		if !ok {
			userDataError(l, 1, id)
		}
		id.Popup(strArg(l, 2))
		return 0
	})
	luaRegister(l, "inputDialogIsDone", func(l *lua.LState) int {
		id, ok := toUserData(l, 1).(InputDialog)
		if !ok {
			userDataError(l, 1, id)
		}
		l.Push(lua.LBool(id.IsDone()))
		return 1
	})
	luaRegister(l, "inputDialogGetStr", func(l *lua.LState) int {
		id, ok := toUserData(l, 1).(InputDialog)
		if !ok {
			userDataError(l, 1, id)
		}
		l.Push(lua.LString(id.GetStr()))
		return 1
	})
	luaRegister(l, "sndPlay", func(l *lua.LState) int {
		s, ok := toUserData(l, 1).(*Snd)
		if !ok {
			userDataError(l, 1, s)
		}
		s.play([...]int32{int32(numArg(l, 2)), int32(numArg(l, 3))})
		return 0
	})
	luaRegister(l, "sndStop", func(l *lua.LState) int {
		s, ok := toUserData(l, 1).(*Snd)
		if !ok {
			userDataError(l, 1, s)
		}
		s.stop([...]int32{int32(numArg(l, 2)), int32(numArg(l, 3))})
		return 0
	})
	luaRegister(l, "playBGM", func(l *lua.LState) int {
		isdefault := true
		var loop, volume, loopstart, loopend int = 1, 100, 0, 0
		if l.GetTop() >= 2 {
			isdefault = boolArg(l, 2)
		}
		if l.GetTop() >= 3 {
			loop = int(numArg(l, 3))
		}
		if l.GetTop() >= 4 {
			volume = int(numArg(l, 4))
		}
		if l.GetTop() >= 5 {
			loopstart = int(numArg(l, 5))
		}
		if l.GetTop() >= 6 {
			loopend = int(numArg(l, 6))
		}
		sys.bgm.Open(strArg(l, 1), isdefault, loop, volume, loopstart, loopend)
		return 0
	})
	luaRegister(l, "esc", func(l *lua.LState) int {
		if l.GetTop() >= 1 {
			sys.esc = boolArg(l, 1)
		}
		l.Push(lua.LBool(sys.esc))
		return 1
	})
	luaRegister(l, "sszRandom", func(l *lua.LState) int {
		l.Push(lua.LNumber(Random()))
		return 1
	})
	luaRegister(l, "setAutoguard", func(l *lua.LState) int {
		pn := int(numArg(l, 1))
		if pn < 1 || pn > MaxSimul*2+MaxAttachedChar {
			l.RaiseError("プレイヤー番号(%v)が不正です。", pn)
		}
		sys.autoguard[pn-1] = boolArg(l, 2)
		return 0
	})
	luaRegister(l, "setPowerShare", func(l *lua.LState) int {
		tn := int(numArg(l, 1))
		if tn < 1 || tn > 2 {
			l.RaiseError("チーム番号(%v)が不正です。", tn)
		}
		sys.powerShare[tn-1] = boolArg(l, 2)
		return 0
	})
	luaRegister(l, "setRoundTime", func(l *lua.LState) int {
		sys.roundTime = int32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "getRoundTime", func(l *lua.LState) int {
		l.Push(lua.LNumber(sys.roundTime))
		return 1
	})
	luaRegister(l, "setHomeTeam", func(l *lua.LState) int {
		tn := int(numArg(l, 1))
		if tn < 1 || tn > 2 {
			l.RaiseError("チーム番号(%v)が不正です。", tn)
		}
		sys.home = tn - 1
		return 0
	})
	luaRegister(l, "setMatchNo", func(l *lua.LState) int {
		sys.match = int32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setConsecutiveWins", func(l *lua.LState) int {
		sys.consecutiveWins[int(numArg(l, 1))-1] = int32(numArg(l, 2))
		return 0
	})
	luaRegister(l, "getConsecutiveWins", func(l *lua.LState) int {
		l.Push(lua.LNumber(sys.consecutiveWins[int(numArg(l, 1))-1]))
		return 1
	})
	luaRegister(l, "setLifeAdjustment", func(l *lua.LState) int {
		sys.lifeAdjustment = boolArg(l, 1)
		return 0
	})
	luaRegister(l, "setLoseKO", func(l *lua.LState) int {
		sys.simulLoseKO = boolArg(l, 1)
		sys.tagLoseKO = boolArg(l, 2)
		return 0
	})
	luaRegister(l, "setMatchWins", func(l *lua.LState) int {
		sys.lifebar.ro.match_wins = int32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "getMatchWins", func(l *lua.LState) int {
		l.Push(lua.LNumber(sys.lifebar.ro.match_wins))
		return 1
	})
	luaRegister(l, "setMatchMaxDrawGames", func(l *lua.LState) int {
		sys.lifebar.ro.match_maxdrawgames = int32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "getMatchMaxDrawGames", func(l *lua.LState) int {
		l.Push(lua.LNumber(sys.lifebar.ro.match_maxdrawgames))
		return 1
	})
	luaRegister(l, "setFramesPerCount", func(l *lua.LState) int {
		sys.lifebar.ti.framespercount = int32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "getFramesPerCount", func(l *lua.LState) int {
		l.Push(lua.LNumber(sys.lifebar.ti.framespercount))
		return 1
	})
	luaRegister(l, "setMasterVolume", func(l *lua.LState) int {
		sys.masterVolume = int(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setBgmVolume", func(l *lua.LState) int {
		sys.bgmVolume = int(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setWavVolume", func(l *lua.LState) int {
		sys.wavVolume = int(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setAudioDucking", func(l *lua.LState) int {
		sys.audioDucking = boolArg(l, 1)
		return 0
	})
	luaRegister(l, "setAllowDebugKeys", func(l *lua.LState) int {
		d := boolArg(l, 1)
		if !d {
			if sys.clsnDraw {
				sys.clsnDraw = false
			}
			if sys.debugDraw {
				sys.debugDraw = false
			}
		}
		sys.allowDebugKeys = d
		return 0
	})
	luaRegister(l, "setGuardBar", func(l *lua.LState) int {
		sys.lifebar.activeGb = boolArg(l, 1)
		return 0
	})
	luaRegister(l, "setStunBar", func(l *lua.LState) int {
		sys.lifebar.activeSb = boolArg(l, 1)
		return 0
	})
	luaRegister(l, "setRedLifeBar", func(l *lua.LState) int {
		sys.lifebar.activeRl = boolArg(l, 1)
		return 0
	})
	
	luaRegister(l, "setLuaLocalcoord", func(l *lua.LState) int {
		sys.luaLocalcoord[0] = int32(numArg(l, 1))
		sys.luaLocalcoord[1] = int32(numArg(l, 2))
		return 0
	})

	// All the lua sprites will be caled by this value
	luaRegister(l, "setLuaSpriteScale", func(l *lua.LState) int {
		sys.luaSpriteScale = float64(numArg(l, 1))
		return 0
	})

	// All the lua sprites will be caled by this value
	luaRegister(l, "setLuaSpriteOffsetX", func(l *lua.LState) int {
		sys.luaSpriteOffsetX = float64(numArg(l, 1))
		return 0
	})

	// All the lua sprites will add this value to his position Y
	luaRegister(l, "setLuaSmallPortraitScale", func(l *lua.LState) int {
		sys.luaSmallPortraitScale = float32(numArg(l, 1))
		return 0
	})
	// All the lua sprites will add this value to his position Y
	luaRegister(l, "setLuaBigPortraitScale", func(l *lua.LState) int {
		sys.luaBigPortraitScale = float32(numArg(l, 1))
		return 0
	})

	// Lifebar localcoord support
	luaRegister(l, "setLuaLifebarScale", func(l *lua.LState) int {
		sys.lifebarScale = float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setLifebarOffsetX", func(l *lua.LState) int {
		sys.lifebarOffsetX = float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setLifebarPortraitScale", func(l *lua.LState) int {
		sys.lifebarPortraitScale = float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setLifebarLocalcoord", func(l *lua.LState) int {
		sys.lifebarLocalcoord[0] = int32(numArg(l, 1))
		sys.lifebarLocalcoord[1] = int32(numArg(l, 2))
		return 0
	})

	// TODO: Test if this even function works.
	luaRegister(l, "setWindowTitle", func(*lua.LState) int {
		sys.windowTitle = string(strArg(l, 1))
		sys.window.SetTitle(sys.windowTitle)
		return 0
	})
}

// System Script

func systemScriptInit(l *lua.LState) {
	scriptCommonInit(l)
	luaRegister(l, "panicError", func(*lua.LState) int {
		l.RaiseError(strArg(l, 1))
		return 0
	})
	luaRegister(l, "bgNew", func(*lua.LState) int {
		s, ok := toUserData(l, 1).(*Sff)
		if !ok {
			userDataError(l, 1, s)
		}
		bg, err := loadBGDef(s, strArg(l, 2), strArg(l, 3))
		if err != nil {
			l.RaiseError(err.Error())
		}
		l.Push(newUserData(l, bg))
		return 1
	})
	luaRegister(l, "bgDraw", func(*lua.LState) int {
		bg, ok := toUserData(l, 1).(*BGDef)
		if !ok {
			userDataError(l, 1, bg)
		}
		top := false
		var x, y, scl float32 = 0, 0, 1
		if l.GetTop() >= 2 {
			top = boolArg(l, 2)
		}
		if l.GetTop() >= 3 {
			x = float32(numArg(l, 3))
		}
		if l.GetTop() >= 4 {
			y = float32(numArg(l, 4))
		}
		if l.GetTop() >= 5 {
			scl = float32(numArg(l, 5))
		}
		bg.draw(top, x, y, scl)
		return 0
	})
	luaRegister(l, "bgReset", func(*lua.LState) int {
		bg, ok := toUserData(l, 1).(*BGDef)
		if !ok {
			userDataError(l, 1, bg)
		}
		bg.reset()
		return 0
	})
	luaRegister(l, "textImgNew", func(*lua.LState) int {
		l.Push(newUserData(l, NewTextSprite()))
		return 1
	})
	luaRegister(l, "textImgSetFont", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		fnt, ok2 := toUserData(l, 2).(*Fnt)
		if !ok2 {
			userDataError(l, 2, fnt)
		}
		ts.fnt = fnt
		return 0
	})
	luaRegister(l, "textImgSetBank", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		ts.bank = int32(numArg(l, 2))
		return 0
	})
	luaRegister(l, "textImgSetAlign", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		ts.align = int32(numArg(l, 2))
		return 0
	})
	luaRegister(l, "textImgSetText", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		ts.text = strArg(l, 2)
		return 0
	})
	luaRegister(l, "textImgSetPos", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		if boolArg(l, 3) {
			ts.x, ts.y = float32((numArg(l, 2)/sys.luaSpriteScale)+sys.luaSpriteOffsetX), float32(numArg(l, 3)/sys.luaSpriteScale)
		}
		return 0
	})
	luaRegister(l, "textImgSetScale", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		ts.xscl, ts.yscl = float32(numArg(l, 2)/sys.luaSpriteScale), float32(numArg(l, 3)/sys.luaSpriteScale)
		return 0
	})
	luaRegister(l, "textImgSetWindow", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		ts.SetWindow(float32((numArg(l, 2)/sys.luaSpriteScale)+sys.luaSpriteOffsetX), float32(numArg(l, 3)/sys.luaSpriteScale),
			float32(numArg(l, 4)/sys.luaSpriteScale), float32(numArg(l, 5)/sys.luaSpriteScale))
		return 0
	})
	luaRegister(l, "textImgSetColor", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		ts.palfx.setColor(float32(numArg(l, 2)), float32(numArg(l, 3)), float32(numArg(l, 4)))
		return 0
	})
	luaRegister(l, "textImgDraw", func(*lua.LState) int {
		ts, ok := toUserData(l, 1).(*TextSprite)
		if !ok {
			userDataError(l, 1, ts)
		}
		ts.Draw()
		return 0
	})
	luaRegister(l, "animNew", func(*lua.LState) int {
		s, ok := toUserData(l, 1).(*Sff)
		if !ok {
			userDataError(l, 1, s)
		}
		act := strArg(l, 2)
		anim := NewAnim(s, act)
		if anim == nil {
			l.RaiseError("\n%v\n\nデータの読み込みに失敗しました。", act)
		}
		l.Push(newUserData(l, anim))
		return 1
	})
	luaRegister(l, "animSetPos", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.SetPos(float32((numArg(l, 2)/sys.luaSpriteScale)+sys.luaSpriteOffsetX), float32(numArg(l, 3)/sys.luaSpriteScale))
		return 0
	})
	luaRegister(l, "animAddPos", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.AddPos(float32(numArg(l, 2)/sys.luaSpriteScale), float32(numArg(l, 3)/sys.luaSpriteScale))
		return 0
	})
	luaRegister(l, "animSetTile", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		var sx, sy int32 = 0, 0
		if l.GetTop() >= 4 {
			sx = int32(numArg(l, 4))
			if l.GetTop() >= 5 {
				sy = int32(numArg(l, 5))
			} else {
				sy = sx
			}
		}
		a.SetTile(int32(numArg(l, 2)), int32(numArg(l, 3)), sx, sy)
		return 0
	})
	luaRegister(l, "animSetColorKey", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.SetColorKey(int16(numArg(l, 2)))
		return 0
	})
	luaRegister(l, "animSetAlpha", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.SetAlpha(int16(numArg(l, 2)), int16(numArg(l, 3)))
		return 0
	})
	luaRegister(l, "animSetScale", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.SetScale(float32(numArg(l, 2)/sys.luaSpriteScale), float32(numArg(l, 3)/sys.luaSpriteScale))
		return 0
	})
	luaRegister(l, "animSetWindow", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.SetWindow(float32((numArg(l, 2)/sys.luaSpriteScale)+sys.luaSpriteOffsetX), float32(numArg(l, 3)/sys.luaSpriteScale),
			float32(numArg(l, 4)/sys.luaSpriteScale), float32(numArg(l, 5)/sys.luaSpriteScale))
		return 0
	})
	luaRegister(l, "animUpdate", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.Update()
		return 0
	})
	luaRegister(l, "animDraw", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.Draw()
		return 0
	})
	luaRegister(l, "animReset", func(*lua.LState) int {
		a, ok := toUserData(l, 1).(*Anim)
		if !ok {
			userDataError(l, 1, a)
		}
		a.ResetFrames()
		return 0
	})
	luaRegister(l, "enterNetPlay", func(*lua.LState) int {
		if sys.netInput != nil {
			l.RaiseError("すでに通信中です。")
		}
		sys.chars = [len(sys.chars)][]*Char{}
		sys.netInput = NewNetInput("save/replays/netplay.replay")
		if host := strArg(l, 1); host != "" {
			sys.netInput.Connect(host, sys.listenPort)
		} else {
			if err := sys.netInput.Accept(sys.listenPort); err != nil {
				l.RaiseError(err.Error())
			}
		}
		return 0
	})
	luaRegister(l, "exitNetPlay", func(*lua.LState) int {
		if sys.netInput != nil {
			sys.netInput.Close()
			sys.netInput = nil
		}
		return 0
	})
	luaRegister(l, "enterReplay", func(*lua.LState) int {
		sys.chars = [len(sys.chars)][]*Char{}
		sys.fileInput = OpenFileInput(strArg(l, 1))
		return 0
	})
	luaRegister(l, "exitReplay", func(*lua.LState) int {
		if sys.fileInput != nil {
			sys.fileInput.Close()
			sys.fileInput = nil
		}
		return 0
	})
	luaRegister(l, "setCom", func(*lua.LState) int {
		pn := int(numArg(l, 1))
		ailv := float32(numArg(l, 2))
		if pn < 1 || pn > MaxSimul*2+MaxAttachedChar {
			l.RaiseError("プレイヤー番号(%v)が不正です。", pn)
		}
		if ailv > 0 {
			sys.com[pn-1] = ailv
		} else {
			sys.com[pn-1] = 0
		}
	
		return 0
	})
	luaRegister(l, "setAutoLevel", func(*lua.LState) int {
		sys.autolevel = boolArg(l, 1)
		return 0
	})
	luaRegister(l, "getListenPort", func(*lua.LState) int {
		l.Push(lua.LString(sys.listenPort))
		return 1
	})
	luaRegister(l, "connected", func(*lua.LState) int {
		l.Push(lua.LBool(sys.netInput.IsConnected()))
		return 1
	})
	luaRegister(l, "setListenPort", func(*lua.LState) int {
		sys.listenPort = strArg(l, 1)
		return 0
	})
	luaRegister(l, "synchronize", func(*lua.LState) int {
		if err := sys.synchronize(); err != nil {
			l.RaiseError(err.Error())
		}
		return 0
	})
	luaRegister(l, "addChar", func(l *lua.LState) int {
		for _, c := range strings.Split(strings.TrimSpace(strArg(l, 1)), "\n") {
			c = strings.Trim(c, "\r")
			if len(c) > 0 {
				sys.sel.addChar(c)
			}
		}
		return 0
	})
	luaRegister(l, "addStage", func(l *lua.LState) int {
		for _, c := range SplitAndTrim(strings.TrimSpace(strArg(l, 1)), "\n") {
			if err := sys.sel.AddStage(c); err != nil {
				l.RaiseError(err.Error())
			}
		}
		return 0
	})
	luaRegister(l, "setRandomSpr", func(*lua.LState) int {
		sff, ok := toUserData(l, 1).(*Sff)
		if !ok {
			userDataError(l, 1, sff)
		}
		sys.sel.randomspr = sff.getOwnPalSprite(int16(numArg(l, 2)),
			int16(numArg(l, 3)))
		sys.sel.randomscl = [...]float32{float32(numArg(l, 4)),
			float32(numArg(l, 5))}
		return 0
	})
	luaRegister(l, "setSelColRow", func(*lua.LState) int {
		sys.sel.columns = int(numArg(l, 1))
		sys.sel.rows = int(numArg(l, 2))
		return 0
	})
	luaRegister(l, "setSelCellSize", func(*lua.LState) int {
		sys.sel.cellsize = [...]float32{float32(numArg(l, 1)),
			float32(numArg(l, 2))}
		return 0
	})
	luaRegister(l, "setSelCellScale", func(*lua.LState) int {
		sys.sel.cellscale = [...]float32{float32(numArg(l, 1)),
			float32(numArg(l, 2))}
		return 0
	})
	luaRegister(l, "numSelCells", func(*lua.LState) int {
		l.Push(lua.LNumber(len(sys.sel.charlist)))
		return 1
	})
	luaRegister(l, "setStage", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.sel.SetStageNo(int(numArg(l, 1)))))
		return 1
	})
	luaRegister(l, "selectStage", func(*lua.LState) int {
		sys.sel.SelectStage(int(numArg(l, 1)))
		return 0
	})
	luaRegister(l, "setTeamMode", func(*lua.LState) int {
		tn := int(numArg(l, 1))
		if tn < 1 || tn > 2 {
			l.RaiseError("The team number (%v) is invalid. / チーム番号(%v)が不正です。", tn)
		}
		tm := TeamMode(numArg(l, 2))
		if tm < 0 || tm > TM_LAST {
			l.RaiseError("The mode number (%v) is invalid. / モード番号(%v)が不正です。", tm)
		}
		nt := int32(numArg(l, 3))
		if nt < 1 || nt > MaxSimul {
			l.RaiseError("The team number (% v) is incorrect. / チーム人数(%v)が不正です。", nt)
		}
		sys.sel.selected[tn-1], sys.tmode[tn-1] = nil, tm
		sys.numTurns[tn-1], sys.numSimul[tn-1] = nt, nt
		if (tm == TM_Simul || tm == TM_Tag) && nt == 1 {
			sys.tmode[tn-1] = TM_Single
		}
		return 0
	})
	luaRegister(l, "setRatioLevel", func(*lua.LState) int {
		pn := int(numArg(l, 1))
		rn := int32(numArg(l, 2))
		if rn < 0 || rn > 4 {
			l.RaiseError("The ratio number (%v) is invalid.", rn)
		}
		sys.ratioLevel[pn-1] = rn
		return 0
	})
	luaRegister(l, "getCharName", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		l.Push(lua.LString(c.name))
		return 1
	})
	luaRegister(l, "getCharFileName", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		l.Push(lua.LString(c.def))
		return 1
	})
	luaRegister(l, "selectChar", func(*lua.LState) int {
		tn := int(numArg(l, 1))
		if tn < 1 || tn > 2 {
			l.RaiseError("チーム番号(%v)が不正です。", tn)
		}
		cn, pl, ret := int(numArg(l, 2)), int(numArg(l, 3)), 0
		if pl >= 1 && pl <= 12 && sys.sel.AddSelectedChar(tn-1, cn, pl) {
			switch sys.tmode[tn-1] {
			case TM_Single:
				ret = 2
			case TM_Simul:
				if len(sys.sel.selected[tn-1]) >= int(sys.numSimul[tn-1]) {
					ret = 2
				} else {
					ret = 1
				}
			case TM_Turns:
				if len(sys.sel.selected[tn-1]) >= int(sys.numTurns[tn-1]) {
					ret = 2
				} else {
					ret = 1
				}
			case TM_Tag:
				if len(sys.sel.selected[tn-1]) >= int(sys.numSimul[tn-1]) {
					ret = 2
				} else {
					ret = 1
				}
			}
		}
		l.Push(lua.LNumber(ret))
		return 1
	})
	luaRegister(l, "getStageName", func(*lua.LState) int {
		l.Push(lua.LString(sys.sel.GetStageName(int(numArg(l, 1)))))
		return 1
	})
	luaRegister(l, "refresh", func(*lua.LState) int {
		sys.playSound()
		if !sys.update() {
			l.RaiseError("<game end>")
		}
		return 0
	})
	luaRegister(l, "drawPortrait", func(l *lua.LState) int {
		n, x, y := int(numArg(l, 1)), float32(numArg(l, 2)), float32(numArg(l, 3))
		var xscl, yscl float32 = 1, 1
		window := &sys.scrrect
		if l.GetTop() >= 4 {
			xscl = float32(numArg(l, 4))
			if l.GetTop() >= 5 {
				yscl = float32(numArg(l, 5))
				if l.GetTop() >= 9 {
					window = &[...]int32{int32(numArg(l, 6)), int32(numArg(l, 7)), int32(numArg(l, 8)), int32(numArg(l, 9))}
				}
			}
		}
		if !sys.frameSkip {
			c := sys.sel.GetChar(n)
			if c != nil {
				if c.lportrait == nil {
					c.loadPortrait(true)
				}
				if c.portrait_scale != 1 {
					xscl *= c.portrait_scale
					yscl *= c.portrait_scale
				}
				paltex := c.lportrait.PalTex
				c.lportrait.Draw(x/float32(sys.luaSpriteScale)+float32(sys.luaSpriteOffsetX), y/float32(sys.luaSpriteScale), xscl/sys.luaBigPortraitScale, yscl/sys.luaBigPortraitScale, c.lportrait.Pal, nil, paltex, window)
			}
		}
		return 0
	})
	luaRegister(l, "drawSmallPortrait", func(l *lua.LState) int {
		n, x, y := int(numArg(l, 1)), float32(numArg(l, 2)), float32(numArg(l, 3))
		var xscl, yscl float32 = 1, 1
		window := &sys.scrrect
		if l.GetTop() >= 4 {
			xscl = float32(numArg(l, 4))
			if l.GetTop() >= 5 {
				yscl = float32(numArg(l, 5))
				if l.GetTop() >= 9 {
					window = &[...]int32{int32(numArg(l, 6)), int32(numArg(l, 7)), int32(numArg(l, 8)), int32(numArg(l, 9))}
				}
			}
		}
		if !sys.frameSkip {
			c := sys.sel.GetChar(n)
			if c != nil {
				if c.sportrait == nil {
					return 0
					//c.loadPortrait(true)
				}
				if c.portrait_scale != 1 {
					xscl *= c.portrait_scale
					yscl *= c.portrait_scale
				}
				paltex := c.sportrait.PalTex
				c.sportrait.Draw(x/float32(sys.luaSpriteScale)+float32(sys.luaSpriteOffsetX), y/float32(sys.luaSpriteScale), xscl/sys.luaSmallPortraitScale, yscl/sys.luaSmallPortraitScale, c.sportrait.Pal, nil, paltex, window)
			}
		}
		return 0
	})
	luaRegister(l, "drawVersusPortrait", func(l *lua.LState) int {
		n, x, y := int(numArg(l, 1)), float32(numArg(l, 2)), float32(numArg(l, 3))
		var xscl, yscl float32 = 1, 1
		window := &sys.scrrect
		if l.GetTop() >= 4 {
			xscl = float32(numArg(l, 4))
			if l.GetTop() >= 5 {
				yscl = float32(numArg(l, 5))
				if l.GetTop() >= 9 {
					window = &[...]int32{int32(numArg(l, 6)), int32(numArg(l, 7)), int32(numArg(l, 8)), int32(numArg(l, 9))}
				}
			}
		}
		if !sys.frameSkip {
			c := sys.sel.GetChar(n)
			if c != nil {
				if c.vsportrait == nil {
					c.loadPortrait(true)
				}
				if c.portrait_scale != 1 {
					xscl *= c.portrait_scale
					yscl *= c.portrait_scale
				}
				paltex := c.vsportrait.PalTex
				c.vsportrait.Draw(x/float32(sys.luaSpriteScale)+float32(sys.luaSpriteOffsetX), y/float32(sys.luaSpriteScale), xscl/sys.luaBigPortraitScale, yscl/sys.luaBigPortraitScale, c.vsportrait.Pal, nil, paltex, window)
			}
		}
		return 0
	})
	luaRegister(l, "drawStagePortrait", func(l *lua.LState) int {
		n, x, y := int(numArg(l, 1)), float32(numArg(l, 2)), float32(numArg(l, 3))
		var xscl, yscl float32 = 1, 1
		window := &sys.scrrect
		if l.GetTop() >= 4 {
			xscl = float32(numArg(l, 4))
			if l.GetTop() >= 5 {
				yscl = float32(numArg(l, 5))
				if l.GetTop() >= 9 {
					window = &[...]int32{int32(numArg(l, 6)), int32(numArg(l, 7)), int32(numArg(l, 8)), int32(numArg(l, 9))}
				}
			}
		}
		if !sys.frameSkip {
			c := sys.sel.GetStage(n)
			if c != nil {
				if c.stageportrait == nil {
					c.loadPortrait()
				}
				if c.portrait_scale != 1 {
					xscl *= c.portrait_scale
					yscl *= c.portrait_scale
				}
				paltex := c.stageportrait.PalTex
				c.stageportrait.Draw(x/float32(sys.luaSpriteScale)+float32(sys.luaSpriteOffsetX), y/float32(sys.luaSpriteScale), xscl/sys.luaBigPortraitScale, yscl/sys.luaBigPortraitScale, c.stageportrait.Pal, nil, paltex, window)
			}
		}
		return 0
	})
	luaRegister(l, "drawFace", func(l *lua.LState) int {
		x, y := float32(numArg(l, 1)), float32(numArg(l, 2))
		offset := 0
		if l.GetTop() >= 3 {
			offset = int(numArg(l, 3))
		}
		if !sys.frameSkip {
			for j := 0; j < sys.sel.rows; j++ {
				for i := 0; i < sys.sel.columns; i++ {
					c := sys.sel.GetChar(offset)
					offset++
					if c != nil {
						if c.sportrait != nil {
							paltex := c.sportrait.PalTex
							c.sportrait.Draw((x + float32(i)*sys.sel.cellsize[0]),
								(y+float32(j)*sys.sel.cellsize[1])/float32(sys.luaSpriteScale),
								(sys.sel.cellscale[0]*c.portrait_scale)/float32(sys.luaSpriteScale),
								(sys.sel.cellscale[1]*c.portrait_scale)/float32(sys.luaSpriteScale),
								c.sportrait.Pal, nil, paltex, &sys.scrrect)
						} else if c.def == "randomselect" && sys.sel.randomspr != nil {
							paltex := sys.sel.randomspr.PalTex
							sys.sel.randomspr.Draw(x+float32(i)*sys.sel.cellsize[0],
								y+float32(j)*sys.sel.cellsize[1]/float32(sys.luaSpriteScale),
								sys.sel.randomscl[0]/float32(sys.luaSpriteScale),
								sys.sel.randomscl[1]/float32(sys.luaSpriteScale),
								sys.sel.randomspr.Pal, nil, paltex, &sys.scrrect)
						}
					}
				}
			}
		}
		return 0
	})
	luaRegister(l, "loadLifebar", func(l *lua.LState) int {
		lb, err := loadLifebar(strArg(l, 1))
		if err != nil {
			l.RaiseError(err.Error())
		}
		sys.lifebar = *lb
		return 0
	})
	luaRegister(l, "loadDebugFont", func(l *lua.LState) int {
		f, err := loadFnt(strArg(l, 1), -1)
		if err != nil {
			l.RaiseError(err.Error())
		}
		sys.debugFont = f
		return 0
	})
	luaRegister(l, "setDebugScript", func(l *lua.LState) int {
		sys.debugScript = strArg(l, 1)
		return 0
	})
	luaRegister(l, "setLifeMul", func(l *lua.LState) int {
		sys.lifeMul = float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setGameSpeed", func(*lua.LState) int {
		if sys.gameSpeed != 100 { //not speedtest
			sys.gameSpeed = float32(numArg(l, 1))
		}
		return 0
	})
	luaRegister(l, "setSingleVsTeamLife", func(l *lua.LState) int {
		sys.singleVsTeamLife = float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setTurnsRecoveryRate", func(l *lua.LState) int {
		sys.turnsRecoveryBase = float32(numArg(l, 1))
		sys.turnsRecoveryBonus = float32(numArg(l, 2))
		return 0
	})
	luaRegister(l, "setZoom", func(l *lua.LState) int {
		sys.cam.ZoomEnable = boolArg(l, 1)
		return 0
	})
	luaRegister(l, "setZoomMin", func(l *lua.LState) int {
		sys.cam.ZoomMin = float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setZoomMax", func(l *lua.LState) int {
		sys.cam.ZoomMax = float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setZoomSpeed", func(l *lua.LState) int {
		sys.cam.ZoomSpeed = 12 - float32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "resetRemapInput", func(l *lua.LState) int {
		sys.resetRemapInput()
		return 0
	})
	luaRegister(l, "remapInput", func(l *lua.LState) int {
		src, dst := int(numArg(l, 1)), int(numArg(l, 2))
		if src < 1 || src > len(sys.inputRemap) ||
			dst < 1 || dst > len(sys.inputRemap) {
			l.RaiseError("プレイヤー番号(%v, %v)が不正です。", src, dst)
		}
		sys.inputRemap[src-1] = dst - 1
		return 0
	})
	luaRegister(l, "loadStart", func(l *lua.LState) int {
		sys.loadStart()
		return 0
	})
	luaRegister(l, "selectStart", func(l *lua.LState) int {
		sys.sel.ClearSelected()
		//sys.loadStart()
		return 0
	})
	luaRegister(l, "overrideCharData", func(l *lua.LState) int {
		pn := int(numArg(l, 1))
		if pn < 1 || pn > MaxSimul*2+MaxAttachedChar {
			l.RaiseError("The player number (%v) is invalid.", pn)
		}
		tableArg(l, 2).ForEach(func(key, value lua.LValue) {
			switch k := key.(type) {
			case lua.LString:
				switch string(k) {
				case "power":
					sys.ocd[pn-1].power = int32(lua.LVAsNumber(value))
				case "guardPoints":
					sys.ocd[pn-1].guardPoints = int32(lua.LVAsNumber(value))
				case "dizzyPoints":
					sys.ocd[pn-1].dizzyPoints = int32(lua.LVAsNumber(value))
				case "life":
					sys.ocd[pn-1].life = int32(lua.LVAsNumber(value))
				case "lifeMax":
					sys.ocd[pn-1].lifeMax = int32(lua.LVAsNumber(value))
				case "lifeRatio":
					sys.ocd[pn-1].lifeRatio = float32(lua.LVAsNumber(value))
				case "attackRatio":
					sys.ocd[pn-1].attackRatio = float32(lua.LVAsNumber(value))
				default:
					l.RaiseError("The table key (%v) is invalid.", key)
				}
			default:
				l.RaiseError("The table key type (%v) is invalid.", fmt.Sprintf("%T\n", key))
			}
		})
		return 0
	})
	luaRegister(l, "setLifeBarTimer", func(*lua.LState) int {
		sys.timerStart = int32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setLifeBarScore", func(*lua.LState) int {
		sys.scoreStart[0] = float32(numArg(l, 1))
		if l.GetTop() >= 2 {
			sys.scoreStart[1] = float32(numArg(l, 2))
		}
		return 0
	})
	luaRegister(l, "setLifeBarElements", func(*lua.LState) int {
		tableArg(l, 1).ForEach(func(key, value lua.LValue) {
			switch k := key.(type) {
			case lua.LString:
				switch string(k) {
				case "timer":
					sys.lifebar.tr.active = lua.LVAsBool(value)
				case "p1score":
					sys.lifebar.sc[0].active = lua.LVAsBool(value)
				case "p2score":
					sys.lifebar.sc[1].active = lua.LVAsBool(value)
				case "match":
					sys.lifebar.ma.active = lua.LVAsBool(value)
				case "p1ai":
					sys.lifebar.ai[0].active = lua.LVAsBool(value)
				case "p2ai":
					sys.lifebar.ai[1].active = lua.LVAsBool(value)
				case "mode":
					sys.lifebar.activeMode = lua.LVAsBool(value)
				case "bars":
					sys.lifebar.activeBars = lua.LVAsBool(value)
				case "lifebar":
					sys.lifebar.active = lua.LVAsBool(value)
				default:
					l.RaiseError("The table key (%v) is invalid.", key)
				}
			default:
				l.RaiseError("The table key type (%v) is invalid.", fmt.Sprintf("%T\n", key))
			}
		})
		return 0
	})
	luaRegister(l, "charChangeState", func(l *lua.LState) int {
		pn := int(numArg(l, 1))
		st := int32(numArg(l, 2))
		if pn >= 1 && pn <= len(sys.chars) && len(sys.chars[pn-1]) > 0 {
			c := sys.chars[pn-1]
			if st == -1 {
				for _, ch := range c {
					ch.setSCF(SCF_disabled)
				}
			} else if c[0].selfStatenoExist(BytecodeInt(st)) == BytecodeBool(true) {
				sys.playerClear(pn-1)
				c[0].changeState(st, -1, -1)
				l.Push(lua.LBool(true))
				return 1
			}
		}
		l.Push(lua.LBool(false))
		return 1
	})
	luaRegister(l, "lastMatchClearance", func(l *lua.LState) int {
		if l.GetTop() == 0 {
			sys.matchClearance = [2]MatchClearance{}
			return 0
		}
		side := int(numArg(l, 1)) - 1
		tableArg(l, 2).ForEach(func(key, value lua.LValue) {
			switch v := value.(type) {
			case lua.LString:
				switch string(v) {
				case "helpers":
					sys.matchClearance[side].helpers = true
				case "sound":
					sys.matchClearance[side].sound = true
				case "projectiles":
					sys.matchClearance[side].projectiles = true
				case "explodes":
					sys.matchClearance[side].explodes = true
				case "fading":
					sys.matchClearance[side].fading = true
				default:
					l.RaiseError("The table value (%v) is invalid.", value)
				}
			default:
				l.RaiseError("The table value type (%v) is invalid.", fmt.Sprintf("%T\n", value))
			}
		})
		sys.matchClearance[side].updated = true
		return 0
	})
	luaRegister(l, "lastMatchClear", func(l *lua.LState) int {
		for i, p := range sys.chars {
			if len(p) > 0 {
				sys.playerClear(i)
			}
		}
		return 0
	})
	luaRegister(l, "lastMatchRender", func(l *lua.LState) int {
		sys.postMatch = true
		lba := sys.lifebar.active
		sys.lifebar.active = boolArg(l, 1)
		x, y, newx, newy, le, ra, scl, sclmul := sys.matchPos[0], sys.matchPos[1], sys.matchPos[2], sys.matchPos[3], sys.matchPos[4], sys.matchPos[5], sys.matchPos[6], sys.matchPos[7]
		//sys.step, sys.roundResetFlg, sys.reloadFlg = false, false, false
		scl = sys.cam.ScaleBound(scl, sclmul)
		tmp := (float32(sys.gameWidth) / 2) / scl
		if AbsF((le+ra)-(newx-x)*2) >= tmp/2 {
			tmp = MaxF(0, MinF(tmp, MaxF((newx-x)-le, ra-(newx-x))))
		}
		x = sys.cam.XBound(scl, MinF(x+le+tmp, MaxF(x+ra-tmp, newx)))
		if !sys.cam.ZoomEnable {
			// Pos X の誤差が出ないように精度を落とす
			x = float32(math.Ceil(float64(x)*4-0.5) / 4)
		}
		y = sys.cam.YBound(scl, newy)
		if sys.tickFrame() && (sys.super <= 0 || !sys.superpausebg) &&
			(sys.pause <= 0 || !sys.pausebg) {
			sys.stage.action()
		}
		newx, newy = x, y
		le, ra, sclmul = sys.action(&newx, &newy, scl)
		if !sys.frameSkip {
			dx, dy, dscl := x, y, scl
			if sys.enableZoomstate {
				if !sys.debugPaused() {
					sys.zoomPosXLag += ((sys.zoomPos[0] - sys.zoomPosXLag) * (1 - sys.zoomlag))
					sys.zoomPosYLag += ((sys.zoomPos[1] - sys.zoomPosYLag) * (1 - sys.zoomlag))
					sys.drawScale = sys.drawScale / (sys.drawScale + (sys.zoomScale*scl-sys.drawScale)*sys.zoomlag) * sys.zoomScale * scl
				}
				dscl = MaxF(sys.cam.MinScale, sys.drawScale/sys.cam.BaseScale())
				dx = sys.cam.XBound(dscl, x+sys.zoomPosXLag/scl)
				dy = y + sys.zoomPosYLag
			} else {
				sys.zoomlag = 0
				sys.zoomPosXLag = 0
				sys.zoomPosYLag = 0
				sys.zoomScale = 1
				sys.zoomPos = [2]float32{0, 0}
				sys.drawScale = sys.cam.Scale
			}
			sys.draw(dx, dy, dscl)
		}
		sys.matchPos = [8]float32{x, y, newx, newy, le, ra, scl, sclmul}
		sys.lifebar.active = lba
		sys.postMatch = false
		return 0
	})
	luaRegister(l, "game", func(l *lua.LState) int {
		load := func() error {
			sys.loader.runTread()
			for sys.loader.state != LS_Complete {
				if sys.loader.state == LS_Error {
					return sys.loader.err
				} else if sys.loader.state == LS_Cancel {
					return nil
				}
				sys.await(FPS)
			}
			for i := range sys.cgi {
				num := len(sys.lifebar.fa[sys.tmode[i&1]])
				if (sys.tmode[i&1] == TM_Simul || sys.tmode[i&1] == TM_Tag) {
					num = int(math.Min(float64(num), float64(sys.numSimul[i&1])*2))
				}
				if i < num {
					ref := sys.tmode[i&1]
					if sys.tmode[i&1] == TM_Simul {
						if sys.numSimul[i&1] == 3 {
							ref = 4
						} else if sys.numSimul[i&1] >= 4 {
							ref = 5
						}
					} else if sys.tmode[i&1] == TM_Tag {
						if sys.numSimul[i&1] == 3 {
							ref = 6
						} else if sys.numSimul[i&1] >= 4 {
							ref = 7
						}
					}
					fa := sys.lifebar.fa[ref][i]
					fa.face = sys.cgi[i].sff.getOwnPalSprite(
						int16(fa.face_spr[0]), int16(fa.face_spr[1]))
					fa.scale = sys.cgi[i].portraitscale
				}
			}
			runtime.GC()
			return nil
		}
		for {
			if sys.gameEnd {
				l.Push(lua.LNumber(-1))
				return 1
			}
			winp := int32(0)
			p := make([]*Char, len(sys.chars))
			sys.roundsExisted = [2]int32{}
			sys.matchWins = [2]int32{}
			for i := range sys.lifebar.wi {
				sys.lifebar.wi[i].clear()
			}
			sys.draws = 0
			tbl := l.NewTable()
			sys.matchData = l.NewTable()
			if !sys.matchClearance[0].updated {
				sys.matchClearance[0] = MatchClearance{}
			}
			if !sys.matchClearance[1].updated {
				sys.matchClearance[1] = MatchClearance{}
			}
			fight := func() (int32, error) {
				if err := load(); err != nil {
					return -1, err
				}
				if sys.loader.state == LS_Cancel {
					return -1, nil
				}
				sys.charList.clear()
				for i := 0; i < len(sys.chars); i += 2 {
					if len(sys.chars[i]) > 0 {
						sys.chars[i][0].id = sys.newCharId()
					}
				}
				for i := 1; i < len(sys.chars); i += 2 {
					if len(sys.chars[i]) > 0 {
						sys.chars[i][0].id = sys.newCharId()
					}
				}
				for i, c := range sys.chars {
					if len(c) > 0 {
						p[i] = c[0]
						sys.charList.add(c[0])
						if sys.roundsExisted[i&1] == 0 {
							c[0].loadPallet()
						}
						for j, cj := range sys.chars {
							if i != j && len(cj) > 0 {
								if len(cj[0].cmd) == 0 {
									cj[0].cmd = make([]CommandList, len(sys.chars))
								}
								cj[0].cmd[i].CopyList(c[0].cmd[i])
							}
						}
					}
				}
				if sys.round == 1 {
					if sys.tmode[1] == TM_Turns {
						sys.matchWins[0] = sys.numTurns[1]
					} else {
						sys.matchWins[0] = sys.lifebar.ro.match_wins
					}
					if sys.tmode[0] == TM_Turns {
						sys.matchWins[1] = sys.numTurns[0]
					} else {
						sys.matchWins[1] = sys.lifebar.ro.match_wins
					}
					sys.stage.reset()
				}
				winp := int32(0)
				if sys.fight() {
					for i, b := range sys.reloadCharSlot {
						if b {
							sys.chars[i] = []*Char{}
							b = false
						}
					}
					sys.loaderReset()
					winp = -2
				} else if sys.esc {
					winp = -1
				} else {
					w1 := sys.wins[0] >= sys.matchWins[0]
					w2 := sys.wins[1] >= sys.matchWins[1]
					if w1 != w2 {
						winp = Btoi(w1) + Btoi(w2)*2
					}
				}
				return winp, nil
			}
			if sys.netInput != nil {
				sys.netInput.Stop()
			}
			defer sys.synchronize()
			for {
				var err error
				if winp, err = fight(); err != nil {
					l.RaiseError(err.Error())
				}
				if winp < 0 || sys.tmode[0] != TM_Turns && sys.tmode[1] != TM_Turns ||
					sys.wins[0] >= sys.matchWins[0] || sys.wins[1] >= sys.matchWins[1] ||
					sys.gameEnd {
					break
				}
				for i := 0; i < 2; i++ {
					if p[i].life <= 0 && sys.tmode[i] == TM_Turns {
						sys.lifebar.fa[TM_Turns][i].numko++
						sys.roundsExisted[i] = 0
					}
				}
				sys.loader.reset()
			}
			if winp != -2 {
				var ti int32
				tbl_time := l.NewTable()
				for k, v := range sys.timerRounds {
					tbl_time.RawSetInt(k+1, lua.LNumber(v))
					ti += v
				}
				sc := sys.scoreStart
				tbl_score := l.NewTable()
				for k, v := range sys.scoreRounds {
					tbl_tmp := l.NewTable()
					tbl_tmp.RawSetInt(1, lua.LNumber(v[0]))
					tbl_tmp.RawSetInt(2, lua.LNumber(v[1]))
					tbl_score.RawSetInt(k+1, tbl_tmp)
					sc[0] += v[0]
					sc[1] += v[1]
				}
				tbl.RawSetString("match", sys.matchData)
				tbl.RawSetString("scoreRounds", tbl_score)
				tbl.RawSetString("timerRounds", tbl_time)
				tbl.RawSetString("time", lua.LNumber(ti))
				tbl.RawSetString("roundTime", lua.LNumber(sys.roundTime))
				tbl.RawSetString("winTeam", lua.LNumber(sys.winTeam))
				tbl.RawSetString("lastRound", lua.LNumber(sys.round-1))
				tbl.RawSetString("draws", lua.LNumber(sys.draws))
				tbl.RawSetString("p1wins", lua.LNumber(sys.wins[0]))
				tbl.RawSetString("p2wins", lua.LNumber(sys.wins[1]))
				tbl.RawSetString("p1tmode", lua.LNumber(sys.tmode[0]))
				tbl.RawSetString("p2tmode", lua.LNumber(sys.tmode[1]))
				tbl.RawSetString("p1score", lua.LNumber(sc[0]))
				tbl.RawSetString("p2score", lua.LNumber(sc[1]))
				tbl.RawSetString("challenger", lua.LNumber(sys.challenger))
				sys.timerStart = 0
				sys.timerRounds = []int32{}
				sys.scoreStart = [2]float32{}
				sys.scoreRounds = [][2]float32{}
				sys.timerCount = []int32{}
				sys.sel.cdefOverwrite = [len(sys.sel.cdefOverwrite)]string{}
				sys.sel.sdefOverwrite = ""
				l.Push(lua.LNumber(winp))
				l.Push(tbl)
				sys.resetGblEffect()
				sys.resetOverrideCharData()
				sys.ratioLevel = [MaxSimul*2 + MaxAttachedChar]int32{}
				sys.matchClearance[0].updated = false
				sys.matchClearance[1].updated = false
				return 2
			}
		}
	})
	luaRegister(l, "getCharVar", func(*lua.LState) int {
		pn := int(numArg(l, 1))
		if pn >= 1 && pn <= len(sys.chars) && len(sys.chars[pn-1]) > 0 {
			if strArg(l, 2) == "varGet" {
				l.Push(lua.LNumber(sys.chars[pn-1][0].varGet(int32(numArg(l, 3))).ToI()))
			} else if strArg(l, 2) == "fvarGet" {
				l.Push(lua.LNumber(sys.chars[pn-1][0].fvarGet(int32(numArg(l, 3))).ToI()))
			} else if strArg(l, 2) == "sysVarGet" {
				l.Push(lua.LNumber(sys.chars[pn-1][0].sysVarGet(int32(numArg(l, 3))).ToI()))
			} else if strArg(l, 2) == "sysFvarGet" {
				l.Push(lua.LNumber(sys.chars[pn-1][0].sysFvarGet(int32(numArg(l, 3))).ToI()))
			} else if strArg(l, 2) == "map" {
				l.Push(lua.LNumber(sys.chars[pn-1][0].mapArray[(strArg(l, 3))]))
			}
		}
		return 1
	})
	luaRegister(l, "getCharVictoryQuote", func(*lua.LState) int {
		pn := int(numArg(l, 1))
		if pn >= 1 && pn <= len(sys.chars) && len(sys.chars[pn-1]) > 0 {
			v := int(-1)
			if l.GetTop() >= 2 {
				v = int(numArg(l, 2))
			} else {
				v = int(sys.chars[pn-1][0].winquote)
			}
			if v < 0 || v >= MaxQuotes {
				t := []int{}
				for i, q := range sys.cgi[sys.chars[pn-1][0].playerNo].quotes {
					if q != "" {
						t = append(t, i)
					}
				}
				if len(t) > 0 {
					v = rand.Int() % len(t)
					v = t[v]
				} else {
					v = -1
				}
			}
			if len(sys.cgi[sys.chars[pn-1][0].playerNo].quotes) == MaxQuotes && v != -1 {
				l.Push(lua.LString(sys.cgi[sys.chars[pn-1][0].playerNo].quotes[v]))
			} else {
				l.Push(lua.LString(""))
			}
		}
		return 1
	})
	luaRegister(l, "getCommandLineFlags", func(*lua.LState) int {
		tbl := l.NewTable()
		for k, v := range sys.cmdFlags {
			tbl.RawSetString(k, lua.LString(v))
		}
		l.Push(tbl)
		return 1
	})
	luaRegister(l, "getCommandLineValue", func(*lua.LState) int {
		l.Push(lua.LString(sys.cmdFlags[strArg(l, 1)]))
		return 1
	})
	luaRegister(l, "setPortrait", func(*lua.LState) int {
		p := int(numArg(l, 3))
		if p == 1 {
			sys.sel.lportrait = [...]int16{int16(numArg(l, 1)), int16(numArg(l, 2))}
		} else if p == 2 {
			sys.sel.sportrait = [...]int16{int16(numArg(l, 1)), int16(numArg(l, 2))}
		} else if p == 3 {
			sys.sel.vsportrait = [...]int16{int16(numArg(l, 1)), int16(numArg(l, 2))}
		} else if p == 4 {
			sys.sel.stageportrait = [...]int16{int16(numArg(l, 1)), int16(numArg(l, 2))}
		}
		return 0
	})
	luaRegister(l, "setMotifDir", func(*lua.LState) int {
		sys.motifDir = strArg(l, 1)
		return 0
	})
	luaRegister(l, "setDemoTime", func(*lua.LState) int {
		sys.demoTime = int32(numArg(l, 1))
		return 0
	})
	luaRegister(l, "setAllowBGM", func(*lua.LState) int {
		sys.allowbgm = boolArg(l, 1)
		return 0
	})
	luaRegister(l, "setGameMode", func(*lua.LState) int {
		sys.gameMode = strArg(l, 1)
		return 0
	})
	luaRegister(l, "setStageBGM", func(l *lua.LState) int {
		k := int(numArg(l, 1))
		sys.sel.stagebgm[k].bgmusic = strArg(l, 2)
		if l.GetTop() >= 3 {
			sys.sel.stagebgm[k].bgmvolume = int32(numArg(l, 3))
		} else {
			sys.sel.stagebgm[k].bgmvolume = 100
		}
		if l.GetTop() >= 4 {
			sys.sel.stagebgm[k].bgmloopstart = int32(numArg(l, 4))
		} else {
			sys.sel.stagebgm[k].bgmloopstart = 0
		}
		if l.GetTop() >= 5 {
			sys.sel.stagebgm[k].bgmloopend = int32(numArg(l, 5))
		} else {
			sys.sel.stagebgm[k].bgmloopend = 0
		}
		return 0
	})
	luaRegister(l, "gameMode", func(*lua.LState) int {
		if l.GetTop() == 0 {
			l.Push(lua.LString(sys.gameMode))
			return 1
		}
		ret := false
		if sys.gameMode == strArg(l, 1) {
			ret = true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "drawCharSprite", func(l *lua.LState) int {
		//pn, spr_tbl (1 or more pairs), x, y, scaleX, scaleY, facing, window
		pn := int(numArg(l, 1))
		if pn < 1 || pn > len(sys.chars) || len(sys.chars[pn-1]) == 0 {
			l.RaiseError("drawCharSprite: the player number (%v) is not loaded.", pn)
		}
		window := &sys.scrrect
		if l.GetTop() >= 11 {
			window = &[...]int32{int32(numArg(l, 8)), int32(numArg(l, 9)), int32(numArg(l, 10)), int32(numArg(l, 11))}
		}
		var ok bool
		var group int16
		tableArg(l, 2).ForEach(func(key, value lua.LValue) {
			if !ok {
				if int(lua.LVAsNumber(key))%2 == 1 {
					group = int16(lua.LVAsNumber(value))
				} else {
					sprite := sys.cgi[pn-1].sff.getOwnPalSprite(group, int16(lua.LVAsNumber(value)))
					if fspr := sprite; fspr != nil {
						pfx := sys.chars[pn-1][0].getPalfx()
						sys.cgi[pn-1].sff.palList.SwapPalMap(&pfx.remap)
						fspr.Pal = nil
						fspr.Pal = fspr.GetPal(&sys.cgi[pn-1].sff.palList)
						sys.cgi[pn-1].sff.palList.SwapPalMap(&pfx.remap)
						lay := Layout{facing: int8(numArg(l, 7)), vfacing: 1, layerno: 1, scale: [...]float32{float32(numArg(l, 5)), float32(numArg(l, 6))}}
						lay.DrawSprite((float32(numArg(l, 3))+sys.lifebarOffsetX)*sys.lifebarScale, float32(numArg(l, 4))*sys.lifebarScale,
							lay.layerno, sprite, pfx, sys.chars[pn-1][0].localscl*sys.cgi[pn-1].portraitscale, window)
						ok = true
					}
				}
			}
		})
		l.Push(lua.LBool(ok))
		return 1
	})
	luaRegister(l, "drawCharAnimation", func(l *lua.LState) int {
		//pn, anim_tbl (1 or more numbers), x, y, scaleX, scaleY, facing, window
		pn := int(numArg(l, 1))
		if pn < 1 || pn > len(sys.chars) || len(sys.chars[pn-1]) == 0 {
			l.RaiseError("drawCharAnimation: the player number (%v) is not loaded.", pn)
		}
		window := &sys.scrrect
		if l.GetTop() >= 11 {
			window = &[...]int32{int32(numArg(l, 8)), int32(numArg(l, 9)), int32(numArg(l, 10)), int32(numArg(l, 11))}
		}
		var ok bool
		tableArg(l, 2).ForEach(func(_, value lua.LValue) {
			if !ok {
				if anim := sys.chars[pn-1][0].getAnim(int32(lua.LVAsNumber(value)), false); anim != nil {
					anim.Action() //TODO: for some reason doesn't advance the animation, remains at first frame
					lay := Layout{facing: int8(numArg(l, 7)), vfacing: 1, layerno: 1, scale: [...]float32{float32(numArg(l, 5)), float32(numArg(l, 6))}}
					lay.DrawAnim(window, float32(numArg(l, 3))+sys.lifebarOffsetX, float32(numArg(l, 4)), sys.chars[pn-1][0].localscl, lay.layerno, anim)
					ok = true
				}
			}
		})
		l.Push(lua.LBool(ok))
		return 1
	})
	luaRegister(l, "resetCharAnimation", func(l *lua.LState) int {
		//pn, anim_tbl (1 or more numbers)
		pn := int(numArg(l, 1))
		if pn < 1 || pn > len(sys.chars) || len(sys.chars[pn-1]) == 0 {
			l.RaiseError("resetCharAnimation: the player number (%v) is not loaded.", pn)
		}
		tableArg(l, 2).ForEach(func(_, value lua.LValue) {
			if anim := sys.chars[pn-1][0].getAnim(int32(lua.LVAsNumber(value)), false); anim != nil {
				anim.Reset()
			}
		})
		return 0
	})
	luaRegister(l, "getCharIntro", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		l.Push(lua.LString(c.intro_storyboard))
		return 1
	})
	luaRegister(l, "getCharEnding", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		l.Push(lua.LString(c.ending_storyboard))
		return 1
	})
	luaRegister(l, "getCharPalettes", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		//palettes
		tbl := l.NewTable()
		if len(c.pal) > 0 {
			for k, v := range c.pal {
				tbl.RawSetInt(k+1, lua.LNumber(v))
			}
		} else {
			tbl.RawSetInt(1, lua.LNumber(1))
		}
		l.Push(tbl)
		//default palettes
		tbl = l.NewTable()
		if len(c.pal_defaults) > 0 {
			for k, v := range c.pal_defaults {
				tbl.RawSetInt(k+1, lua.LNumber(v))
			}
		} else {
			tbl.RawSetInt(1, lua.LNumber(1))
		}
		l.Push(tbl)
		//palette keymap
		tbl = l.NewTable()
		if len(c.pal_keymap) > 0 {
			for k, v := range c.pal_keymap {
				if int32(k + 1) != v { //only actual remaps are relevant
					tbl.RawSetInt(k+1, lua.LNumber(v))
				}
			}
		}
		l.Push(tbl)
		return 3
	})
	luaRegister(l, "getCharRandomPalette", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		if len(c.pal) > 0 {
			n := rand.Int() % len(c.pal)
			l.Push(lua.LNumber(c.pal[n]))
		} else {
			l.Push(lua.LNumber(1))
		}
		return 1
	})
	luaRegister(l, "getCharSff", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		l.Push(lua.LString(c.sprite))
		return 1
	})
	luaRegister(l, "getCharSnd", func(*lua.LState) int {
		c := sys.sel.GetChar(int(numArg(l, 1)))
		l.Push(lua.LString(c.sound))
		return 1
	})
	luaRegister(l, "getAttachedCharInfo", func(*lua.LState) int {
		def := strArg(l, 1)
		idx := strings.Index(def, "/")
		if len(def) >= 4 && strings.ToLower(def[len(def)-4:]) == ".def" {
			if idx < 0 {
				return 0
			}
		} else if idx < 0 {
			def += "/" + def + ".def"
		} else {
			def += ".def"
		}
		if strings.ToLower(def[0:6]) != "chars/" && strings.ToLower(def[1:3]) != ":/" && (def[0] != '/' || idx > 0 && strings.Index(def[:idx], ":") < 0) {
			def = "chars/" + def
		}
		if def = FileExist(def); len(def) == 0 {
			return 0
		}
		str, err := LoadText(def)
		if err != nil {
			return 0
		}
		lines, i, info, files, displayname, sprite, sound := SplitAndTrim(str, "\n"), 0, true, true, "", "", ""
		for i < len(lines) {
			is, name, _ := ReadIniSection(lines, &i)
			switch name {
			case "info":
				if info {
					info = false
					var ok bool
					displayname, ok, _ = is.getText("displayname")
					if !ok {
						displayname, _, _ = is.getText("name")
					}
				}
			case "files":
				if files {
					files = false
					sprite = is["sprite"]
					sound = is["sound"]
				}
			}
		}
		l.Push(lua.LString(def))
		l.Push(lua.LString(displayname))
		l.Push(lua.LString(sprite))
		l.Push(lua.LString(sound))
		return 4
	})
	luaRegister(l, "getWaveData", func(*lua.LState) int {
		//path, group, sound, loops before give up searching for group/sound pair (optional)
		var max uint32
		if l.GetTop() >= 4 {
			max = uint32(numArg(l, 4))
		}
		w, err := loadFromSnd(strArg(l, 1), int32(numArg(l, 2)), int32(numArg(l, 3)), max)
		if err != nil {
			l.RaiseError(err.Error())
		}
		l.Push(newUserData(l, w))
		return 1
	})
	luaRegister(l, "wavePlay", func(l *lua.LState) int {
		w, ok := toUserData(l, 1).(*Wave)
		if !ok {
			userDataError(l, 1, w)
		}
		w.play()
		return 0
	})
	luaRegister(l, "stopAllSounds", func(l *lua.LState) int {
		sys.sounds = newSounds(len(sys.sounds))
		return 0
	})
	luaRegister(l, "getSoundPlaying", func(*lua.LState) int {
		s, ok := toUserData(l, 1).(*Snd)
		if !ok {
			userDataError(l, 1, s)
		}
		w := s.Get([...]int32{int32(numArg(l, 2)), int32(numArg(l, 3))})
		var found bool
		for _, v := range sys.sounds {
			if v.sound != nil && v.sound == w {
				found = true
				break
			}
		}
		l.Push(lua.LBool(found))
		return 1
	})
	luaRegister(l, "getStageInfo", func(*lua.LState) int {
		stagebgm, attachedchardef := sys.sel.GetStageInfo(int(numArg(l, 1)))
		tbl := l.NewTable()
		for k, v := range stagebgm {
			subt := l.NewTable()
			subt.RawSetString("bgmusic", lua.LString(v.bgmusic))
			subt.RawSetString("bgmvolume", lua.LNumber(v.bgmvolume))
			subt.RawSetString("bgmloopstart", lua.LNumber(v.bgmloopstart))
			subt.RawSetString("bgmloopend", lua.LNumber(v.bgmloopend))
			tbl.RawSetInt(k + 1, subt)
		}
		l.Push(tbl)
		l.Push(lua.LString(attachedchardef))
		return 2
	})
	luaRegister(l, "getGamepadName", func(*lua.LState) int {
		l.Push(lua.LString(joystick[int(numArg(l, 1))].GetGamepadName()))
		return 1
	})
	luaRegister(l, "getJoystickPresent", func(*lua.LState) int {
		joy := int(numArg(l, 1))
		present := joystick[joy].Present()
		l.Push(lua.LBool(present))
		return 1
	})
	luaRegister(l, "getKey", func(*lua.LState) int {
		s := ""
		if sys.keyInput != glfw.KeyUnknown {
			s = KeyToString(sys.keyInput)
		}
		for j := 0; j < 1; j++ {
			if joystick[j].GetGamepadState != nil {
				axes := joystick[j].GetAxes()
				btns := joystick[j].GetButtons()
				for i := range axes {
					if strings.Contains(joystick[j].GetGamepadName(), "XInput") { //Xbox360コントローラー判定
						if axes[i] > 0.5 {
							s = strconv.Itoa(-i*2 - 2)
						} else if axes[i] < -0.5 && i < 4 {
							s = strconv.Itoa(-i*2 - 1)
						}
					} else {
						// PS4 Controller support
						if joystick[j].GetGamepadName() != "PS4 Controller" || !(i == 3 || i == 4) {
							if axes[i] < -0.2 {
								s = strconv.Itoa(-i*2 - 1)
							} else if axes[i] > 0.2 {
								s = strconv.Itoa(-i*2 - 2)
							}
						}
					}
				}
				for i := range btns {
					if btns[i] > 0 {
						s = strconv.Itoa(i)
					}
				}
			}
		}

		l.Push(lua.LString(s))
		return 1
	})
	luaRegister(l, "getKeyText", func(*lua.LState) int {
		s := ""
		if sys.keyInput != glfw.KeyUnknown {
			if sys.keyInput == glfw.KeyInsert {
				s = sys.window.GetClipboardString()
			} else {
				s = sys.keyString
			}
		}
		l.Push(lua.LString(s))
		return 1
	})
	luaRegister(l, "resetKey", func(*lua.LState) int {
		sys.keyInput = glfw.KeyUnknown
		sys.keyString = ""
		return 0
	})
	luaRegister(l, "getSpriteInfo", func(*lua.LState) int {
		var s *Sprite
		var err error
		def := strArg(l, 1)
		err = LoadFile(&def, "", func(file string) error {
			s, err = loadFromSff(file, int16(numArg(l, 2)), int16(numArg(l, 3)))
			return err
		})
		if err != nil {
			l.Push(lua.LNumber(0))
			l.Push(lua.LNumber(0))
			l.Push(lua.LNumber(0))
			l.Push(lua.LNumber(0))
			return 4
		}
		l.Push(lua.LNumber(s.Size[0]))
		l.Push(lua.LNumber(s.Size[1]))
		l.Push(lua.LNumber(s.Offset[0]))
		l.Push(lua.LNumber(s.Offset[1]))
		return 4
	})
	luaRegister(l, "toggleStatusDraw", func(*lua.LState) int {
		sys.statusDraw = !sys.statusDraw
		return 0
	})
	luaRegister(l, "toggleMaxPowerMode", func(*lua.LState) int {
		sys.maxPowerMode = !sys.maxPowerMode
		return 0
	})
}

// Trigger Script

func triggerScriptInit(l *lua.LState) {
	sys.debugWC = sys.chars[0][0]
	luaRegister(l, "player", func(*lua.LState) int {
		pn := int(numArg(l, 1))
		ret := false
		if pn >= 1 && pn <= len(sys.chars) && len(sys.chars[pn-1]) > 0 {
			sys.debugWC, ret = sys.chars[pn-1][0], true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "parent", func(*lua.LState) int {
		ret := false
		if c := sys.debugWC.parent(); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "root", func(*lua.LState) int {
		ret := false
		if c := sys.debugWC.root(); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "helper", func(*lua.LState) int {
		ret, id := false, int32(0)
		if l.GetTop() >= 1 {
			id = int32(numArg(l, 1))
		}
		if c := sys.debugWC.helper(id); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "target", func(*lua.LState) int {
		ret, id := false, int32(-1)
		if l.GetTop() >= 1 {
			id = int32(numArg(l, 1))
		}
		if c := sys.debugWC.target(id); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "partner", func(*lua.LState) int {
		ret := false
		if c := sys.debugWC.partner(0); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "enemy", func(*lua.LState) int {
		ret, n := false, int32(0)
		if l.GetTop() >= 1 {
			n = int32(numArg(l, 1))
		}
		if c := sys.debugWC.enemy(n); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "enemynear", func(*lua.LState) int {
		ret, n := false, int32(0)
		if l.GetTop() >= 1 {
			n = int32(numArg(l, 1))
		}
		if c := sys.debugWC.enemyNear(n); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "playerid", func(*lua.LState) int {
		ret := false
		if c := sys.playerID(int32(numArg(l, 1))); c != nil {
			sys.debugWC, ret = c, true
		}
		l.Push(lua.LBool(ret))
		return 1
	})
	luaRegister(l, "ailevel", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.aiLevel()))
		return 1
	})
	luaRegister(l, "alive", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.alive()))
		return 1
	})
	luaRegister(l, "anim", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.animNo))
		return 1
	})
	luaRegister(l, "animOwner", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.animPN) + 1)
		return 1
	})
	luaRegister(l, "animelemno", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.animElemNo(int32(numArg(l, 1))).ToI()))
		return 1
	})
	luaRegister(l, "animelemtime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.animElemTime(int32(numArg(l, 1))).ToI()))
		return 1
	})
	luaRegister(l, "animexist", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.animExist(sys.debugWC,
			BytecodeInt(int32(numArg(l, 1)))).ToB()))
		return 1
	})
	luaRegister(l, "animtime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.animTime()))
		return 1
	})
	luaRegister(l, "authorname", func(*lua.LState) int {
		l.Push(lua.LString(sys.debugWC.gi().author))
		return 1
	})
	luaRegister(l, "backedge", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.backEdge()))
		return 1
	})
	luaRegister(l, "backedgebodydist", func(*lua.LState) int {
		l.Push(lua.LNumber(int32(sys.debugWC.backEdgeBodyDist())))
		return 1
	})
	luaRegister(l, "backedgedist", func(*lua.LState) int {
		l.Push(lua.LNumber(int32(sys.debugWC.backEdgeDist())))
		return 1
	})
	luaRegister(l, "bottomedge", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.bottomEdge()))
		return 1
	})
	luaRegister(l, "cameraposX", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.cam.Pos[0]))
		return 1
	})
	luaRegister(l, "cameraposY", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.cam.Pos[1]))
		return 1
	})
	luaRegister(l, "camerazoom", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.cam.Scale))
		return 1
	})
	luaRegister(l, "canrecover", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.canRecover()))
		return 1
	})
	luaRegister(l, "command", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.commandByName(strArg(l, 1))))
		return 1
	})
	luaRegister(l, "const", func(*lua.LState) int {
		c := sys.debugWC
		var ln lua.LNumber
		switch strArg(l, 1) {
		case "data.life":
			ln = lua.LNumber(c.gi().data.life)
		case "data.power":
			ln = lua.LNumber(c.gi().data.power)
		case "data.guardpoints":
			ln = lua.LNumber(c.gi().data.guardpoints)
		case "data.dizzypoints":
			ln = lua.LNumber(c.gi().data.dizzypoints)
		case "data.attack":
			ln = lua.LNumber(c.gi().data.attack)
		case "data.defence":
			ln = lua.LNumber(c.gi().data.defence)
		case "data.fall.defence_mul":
			ln = lua.LNumber(c.gi().data.fall.defence_mul)
		case "data.liedown.time":
			ln = lua.LNumber(c.gi().data.liedown.time)
		case "data.airjuggle":
			ln = lua.LNumber(c.gi().data.airjuggle)
		case "data.sparkno":
			ln = lua.LNumber(c.gi().data.sparkno)
		case "data.guard.sparkno":
			ln = lua.LNumber(c.gi().data.guard.sparkno)
		case "data.ko.echo":
			ln = lua.LNumber(c.gi().data.ko.echo)
		case "data.intpersistindex":
			ln = lua.LNumber(c.gi().data.intpersistindex)
		case "data.floatpersistindex":
			ln = lua.LNumber(c.gi().data.floatpersistindex)
		case "size.xscale":
			ln = lua.LNumber(c.size.xscale)
		case "size.yscale":
			ln = lua.LNumber(c.size.yscale)
		case "size.ground.back":
			ln = lua.LNumber(c.size.ground.back)
		case "size.ground.front":
			ln = lua.LNumber(c.size.ground.front)
		case "size.air.back":
			ln = lua.LNumber(c.size.air.back)
		case "size.air.front":
			ln = lua.LNumber(c.size.air.front)
		case "size.z.width":
			ln = lua.LNumber(c.size.z.width)
		case "size.height":
			ln = lua.LNumber(c.size.height)
		case "size.attack.dist":
			ln = lua.LNumber(c.size.attack.dist)
		case "size.attack.z.width.back":
			ln = lua.LNumber(c.size.attack.z.width[1])
		case "size.attack.z.width.front":
			ln = lua.LNumber(c.size.attack.z.width[0])
		case "size.proj.attack.dist":
			ln = lua.LNumber(c.size.proj.attack.dist)
		case "size.proj.doscale":
			ln = lua.LNumber(c.size.proj.doscale)
		case "size.head.pos.x":
			ln = lua.LNumber(c.size.head.pos[0])
		case "size.head.pos.y":
			ln = lua.LNumber(c.size.head.pos[1])
		case "size.mid.pos.x":
			ln = lua.LNumber(c.size.mid.pos[0])
		case "size.mid.pos.y":
			ln = lua.LNumber(c.size.mid.pos[1])
		case "size.shadowoffset":
			ln = lua.LNumber(c.size.shadowoffset)
		case "size.draw.offset.x":
			ln = lua.LNumber(c.size.draw.offset[0])
		case "size.draw.offset.y":
			ln = lua.LNumber(c.size.draw.offset[1])
		case "velocity.walk.fwd.x":
			ln = lua.LNumber(c.gi().velocity.walk.fwd)
		case "velocity.walk.back.x":
			ln = lua.LNumber(c.gi().velocity.walk.back)
		case "velocity.walk.up.x":
			ln = lua.LNumber(c.gi().velocity.walk.up.x)
		case "velocity.walk.down.x":
			ln = lua.LNumber(c.gi().velocity.walk.down.x)
		case "velocity.run.fwd.x":
			ln = lua.LNumber(c.gi().velocity.run.fwd[0])
		case "velocity.run.fwd.y":
			ln = lua.LNumber(c.gi().velocity.run.fwd[1])
		case "velocity.run.back.x":
			ln = lua.LNumber(c.gi().velocity.run.back[0])
		case "velocity.run.back.y":
			ln = lua.LNumber(c.gi().velocity.run.back[1])
		case "velocity.run.up.x":
			ln = lua.LNumber(c.gi().velocity.run.up.x)
		case "velocity.run.up.y":
			ln = lua.LNumber(c.gi().velocity.run.up.y)
		case "velocity.run.down.x":
			ln = lua.LNumber(c.gi().velocity.run.down.x)
		case "velocity.run.down.y":
			ln = lua.LNumber(c.gi().velocity.run.down.y)
		case "velocity.jump.y":
			ln = lua.LNumber(c.gi().velocity.jump.neu[1])
		case "velocity.jump.neu.x":
			ln = lua.LNumber(c.gi().velocity.jump.neu[0])
		case "velocity.jump.back.x":
			ln = lua.LNumber(c.gi().velocity.jump.back)
		case "velocity.jump.fwd.x":
			ln = lua.LNumber(c.gi().velocity.jump.fwd)
		case "velocity.jump.up.x":
			ln = lua.LNumber(c.gi().velocity.jump.up.x)
		case "velocity.jump.down.x":
			ln = lua.LNumber(c.gi().velocity.jump.down.x)
		case "velocity.runjump.back.x":
			ln = lua.LNumber(c.gi().velocity.runjump.back[0])
		case "velocity.runjump.back.y":
			ln = lua.LNumber(c.gi().velocity.runjump.back[1])
		case "velocity.runjump.y":
			ln = lua.LNumber(c.gi().velocity.runjump.fwd[1])
		case "velocity.runjump.fwd.x":
			ln = lua.LNumber(c.gi().velocity.runjump.fwd[0])
		case "velocity.runjump.up.x":
			ln = lua.LNumber(c.gi().velocity.runjump.up.x)
		case "velocity.runjump.down.x":
			ln = lua.LNumber(c.gi().velocity.runjump.down.x)
		case "velocity.airjump.y":
			ln = lua.LNumber(c.gi().velocity.airjump.neu[1])
		case "velocity.airjump.neu.x":
			ln = lua.LNumber(c.gi().velocity.airjump.neu[0])
		case "velocity.airjump.back.x":
			ln = lua.LNumber(c.gi().velocity.airjump.back)
		case "velocity.airjump.fwd.x":
			ln = lua.LNumber(c.gi().velocity.airjump.fwd)
		case "velocity.airjump.up.x":
			ln = lua.LNumber(c.gi().velocity.airjump.up.x)
		case "velocity.airjump.down.x":
			ln = lua.LNumber(c.gi().velocity.airjump.down.x)
		case "velocity.air.gethit.groundrecover.x":
			ln = lua.LNumber(c.gi().velocity.air.gethit.groundrecover[0])
		case "velocity.air.gethit.groundrecover.y":
			ln = lua.LNumber(c.gi().velocity.air.gethit.groundrecover[1])
		case "velocity.air.gethit.airrecover.mul.x":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.mul[0])
		case "velocity.air.gethit.airrecover.mul.y":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.mul[1])
		case "velocity.air.gethit.airrecover.add.x":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.add[0])
		case "velocity.air.gethit.airrecover.add.y":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.add[1])
		case "velocity.air.gethit.airrecover.back":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.back)
		case "velocity.air.gethit.airrecover.fwd":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.fwd)
		case "velocity.air.gethit.airrecover.up":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.up)
		case "velocity.air.gethit.airrecover.down":
			ln = lua.LNumber(c.gi().velocity.air.gethit.airrecover.down)
		case "movement.airjump.num":
			ln = lua.LNumber(c.gi().movement.airjump.num)
		case "movement.airjump.height":
			ln = lua.LNumber(c.gi().movement.airjump.height)
		case "movement.yaccel":
			ln = lua.LNumber(c.gi().movement.yaccel)
		case "movement.stand.friction":
			ln = lua.LNumber(c.gi().movement.stand.friction)
		case "movement.crouch.friction":
			ln = lua.LNumber(c.gi().movement.crouch.friction)
		case "movement.stand.friction.threshold":
			ln = lua.LNumber(c.gi().movement.stand.friction_threshold)
		case "movement.crouch.friction.threshold":
			ln = lua.LNumber(c.gi().movement.crouch.friction_threshold)
		case "movement.air.gethit.groundlevel":
			ln = lua.LNumber(c.gi().movement.air.gethit.groundlevel)
		case "movement.air.gethit.groundrecover.ground.threshold":
			ln = lua.LNumber(
				c.gi().movement.air.gethit.groundrecover.ground.threshold)
		case "movement.air.gethit.groundrecover.groundlevel":
			ln = lua.LNumber(c.gi().movement.air.gethit.groundrecover.groundlevel)
		case "movement.air.gethit.airrecover.threshold":
			ln = lua.LNumber(c.gi().movement.air.gethit.airrecover.threshold)
		case "movement.air.gethit.airrecover.yaccel":
			ln = lua.LNumber(c.gi().movement.air.gethit.airrecover.yaccel)
		case "movement.air.gethit.trip.groundlevel":
			ln = lua.LNumber(c.gi().movement.air.gethit.trip.groundlevel)
		case "movement.down.bounce.offset.x":
			ln = lua.LNumber(c.gi().movement.down.bounce.offset[0])
		case "movement.down.bounce.offset.y":
			ln = lua.LNumber(c.gi().movement.down.bounce.offset[1])
		case "movement.down.bounce.yaccel":
			ln = lua.LNumber(c.gi().movement.down.bounce.yaccel)
		case "movement.down.bounce.groundlevel":
			ln = lua.LNumber(c.gi().movement.down.bounce.groundlevel)
		case "movement.down.friction.threshold":
			ln = lua.LNumber(c.gi().movement.down.friction_threshold)
		}
		l.Push(ln)
		return 1
	})
	luaRegister(l, "ctrl", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.ctrl()))
		return 1
	})
	luaRegister(l, "drawgame", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.drawgame()))
		return 1
	})
	luaRegister(l, "facing", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.facing))
		return 1
	})
	luaRegister(l, "frontedge", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.frontEdge()))
		return 1
	})
	luaRegister(l, "frontedgebodydist", func(*lua.LState) int {
		l.Push(lua.LNumber(int32(sys.debugWC.frontEdgeBodyDist())))
		return 1
	})
	luaRegister(l, "frontedgedist", func(*lua.LState) int {
		l.Push(lua.LNumber(int32(sys.debugWC.frontEdgeDist())))
		return 1
	})
	luaRegister(l, "fvar", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.fvarGet(int32(numArg(l, 1))).ToF()))
		return 1
	})
	luaRegister(l, "gameheight", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.gameHeight()))
		return 1
	})
	luaRegister(l, "gametime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.gameTime))
		return 1
	})
	luaRegister(l, "gamemode", func(*lua.LState) int {
		l.Push(lua.LString(sys.gameMode))
		return 1
	})
	luaRegister(l, "gamewidth", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.gameWidth()))
		return 1
	})
	luaRegister(l, "gethitvar", func(*lua.LState) int {
		c := sys.debugWC
		var ln lua.LNumber
		switch strArg(l, 1) {
		case "xveladd":
			ln = lua.LNumber(0)
		case "yveladd":
			ln = lua.LNumber(0)
		case "type":
			ln = lua.LNumber(0)
		case "zoff":
			ln = lua.LNumber(0)
		case "fall.envshake.dir":
			ln = lua.LNumber(0)
		case "animtype":
			ln = lua.LNumber(c.gethitAnimtype())
		case "airtype":
			ln = lua.LNumber(c.ghv.airtype)
		case "groundtype":
			ln = lua.LNumber(c.ghv.groundtype)
		case "damage":
			ln = lua.LNumber(c.ghv.damage)
		case "hitcount":
			ln = lua.LNumber(c.ghv.hitcount)
		case "fallcount":
			ln = lua.LNumber(c.ghv.fallcount)
		case "hitshaketime":
			ln = lua.LNumber(c.ghv.hitshaketime)
		case "hittime":
			ln = lua.LNumber(c.ghv.hittime)
		case "slidetime":
			ln = lua.LNumber(c.ghv.slidetime)
		case "ctrltime":
			ln = lua.LNumber(c.ghv.ctrltime)
		case "recovertime":
			ln = lua.LNumber(c.recoverTime)
		case "xoff":
			ln = lua.LNumber(c.ghv.xoff)
		case "yoff":
			ln = lua.LNumber(c.ghv.yoff)
		case "xvel":
			ln = lua.LNumber(c.ghv.xvel * c.facing)
		case "yvel":
			ln = lua.LNumber(c.ghv.yvel)
		case "yaccel":
			ln = lua.LNumber(c.ghv.getYaccel(c))
		case "hitid", "chainid":
			ln = lua.LNumber(c.ghv.chainId())
		case "guarded":
			ln = lua.LNumber(Btoi(c.ghv.guarded))
		case "isbound":
			ln = lua.LNumber(Btoi(c.isBound()))
		case "fall":
			ln = lua.LNumber(Btoi(c.ghv.fallf))
		case "fall.damage":
			ln = lua.LNumber(c.ghv.fall.damage)
		case "fall.xvel":
			ln = lua.LNumber(c.ghv.fall.xvel())
		case "fall.yvel":
			ln = lua.LNumber(c.ghv.fall.yvelocity)
		case "fall.recover":
			ln = lua.LNumber(Btoi(c.ghv.fall.recover))
		case "fall.time":
			ln = lua.LNumber(c.fallTime)
		case "fall.recovertime":
			ln = lua.LNumber(c.ghv.fall.recovertime)
		case "fall.kill":
			ln = lua.LNumber(Btoi(c.ghv.fall.kill))
		case "fall.envshake.time":
			ln = lua.LNumber(c.ghv.fall.envshake_time)
		case "fall.envshake.freq":
			ln = lua.LNumber(c.ghv.fall.envshake_freq)
		case "fall.envshake.ampl":
			ln = lua.LNumber(c.ghv.fall.envshake_ampl)
		case "fall.envshake.phase":
			ln = lua.LNumber(c.ghv.fall.envshake_phase)
		case "score":
			ln = lua.LNumber(c.ghv.score)
		}
		l.Push(ln)
		return 1
	})
	luaRegister(l, "hitcount", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.hitCount))
		return 1
	})
	luaRegister(l, "hitdefattr", func(*lua.LState) int {
		attr, str := sys.debugWC.hitdef.attr, ""
		if sys.debugWC.ss.moveType == MT_A {
			if attr&int32(ST_S) != 0 {
				str += "S"
			}
			if attr&int32(ST_C) != 0 {
				str += "C"
			}
			if attr&int32(ST_A) != 0 {
				str += "A"
			}
			if attr&int32(AT_NA) != 0 {
				str += ", NA"
			}
			if attr&int32(AT_NT) != 0 {
				str += ", NT"
			}
			if attr&int32(AT_NP) != 0 {
				str += ", NP"
			}
			if attr&int32(AT_SA) != 0 {
				str += ", SA"
			}
			if attr&int32(AT_ST) != 0 {
				str += ", ST"
			}
			if attr&int32(AT_SP) != 0 {
				str += ", SP"
			}
			if attr&int32(AT_HA) != 0 {
				str += ", HA"
			}
			if attr&int32(AT_HT) != 0 {
				str += ", HT"
			}
			if attr&int32(AT_HP) != 0 {
				str += ", HP"
			}
		}
		l.Push(lua.LString(str))
		return 1
	})
	luaRegister(l, "hitfall", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.ghv.fallf))
		return 1
	})
	luaRegister(l, "hitover", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.hitOver()))
		return 1
	})
	luaRegister(l, "hitpausetime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.hitPauseTime))
		return 1
	})
	luaRegister(l, "hitshakeover", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.hitShakeOver()))
		return 1
	})
	luaRegister(l, "hitvelX", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.hitVelX()))
		return 1
	})
	luaRegister(l, "hitvelY", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.hitVelY()))
		return 1
	})
	luaRegister(l, "id", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.id))
		return 1
	})
	luaRegister(l, "inguarddist", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.inguarddist))
		return 1
	})
	luaRegister(l, "ishelper", func(*lua.LState) int {
		id := int32(0)
		if l.GetTop() >= 1 {
			id = int32(numArg(l, 1))
		}
		l.Push(lua.LBool(sys.debugWC.isHelper(BytecodeInt(id)).ToB()))
		return 1
	})
	luaRegister(l, "ishometeam", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.teamside == sys.home))
		return 1
	})
	luaRegister(l, "leftedge", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.leftEdge()))
		return 1
	})
	luaRegister(l, "life", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.life))
		return 1
	})
	luaRegister(l, "lifemax", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.lifeMax))
		return 1
	})
	luaRegister(l, "lose", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.lose()))
		return 1
	})
	luaRegister(l, "loseko", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.loseKO()))
		return 1
	})
	luaRegister(l, "losetime", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.loseTime()))
		return 1
	})
	luaRegister(l, "matchno", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.match))
		return 1
	})
	luaRegister(l, "matchover", func(*lua.LState) int {
		l.Push(lua.LBool(sys.matchOver()))
		return 1
	})
	luaRegister(l, "movecontact", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.moveContact()))
		return 1
	})
	luaRegister(l, "moveguarded", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.moveGuarded()))
		return 1
	})
	luaRegister(l, "movehit", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.moveHit()))
		return 1
	})
	luaRegister(l, "movereversed", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.moveReversed()))
		return 1
	})
	luaRegister(l, "movetype", func(*lua.LState) int {
		var s string
		switch sys.debugWC.ss.moveType {
		case MT_I:
			s = "I"
		case MT_A:
			s = "A"
		case MT_H:
			s = "H"
		}
		l.Push(lua.LString(s))
		return 1
	})
	luaRegister(l, "name", func(*lua.LState) int {
		l.Push(lua.LString(sys.debugWC.name))
		return 1
	})
	luaRegister(l, "numenemy", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.numEnemy()))
		return 1
	})
	luaRegister(l, "numexplod", func(*lua.LState) int {
		id := int32(-1)
		if l.GetTop() >= 1 {
			id = int32(numArg(l, 1))
		}
		l.Push(lua.LNumber(sys.debugWC.numExplod(BytecodeInt(id)).ToI()))
		return 1
	})
	luaRegister(l, "numhelper", func(*lua.LState) int {
		id := int32(0)
		if l.GetTop() >= 1 {
			id = int32(numArg(l, 1))
		}
		l.Push(lua.LNumber(sys.debugWC.numHelper(BytecodeInt(id)).ToI()))
		return 1
	})
	luaRegister(l, "numpartner", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.numPartner()))
		return 1
	})
	luaRegister(l, "numproj", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.numProj()))
		return 1
	})
	luaRegister(l, "numprojid", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.numProjID(
			BytecodeInt(int32(numArg(l, 1)))).ToI()))
		return 1
	})
	luaRegister(l, "numtarget", func(*lua.LState) int {
		id := int32(-1)
		if l.GetTop() >= 1 {
			id = int32(numArg(l, 1))
		}
		l.Push(lua.LNumber(sys.debugWC.numTarget(BytecodeInt(id)).ToI()))
		return 1
	})
	luaRegister(l, "palno", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.gi().palno))
		return 1
	})
	luaRegister(l, "physics", func(*lua.LState) int {
		var s string
		switch sys.debugWC.ss.physics {
		case ST_S:
			s = "S"
		case ST_C:
			s = "C"
		case ST_A:
			s = "A"
		case ST_N:
			s = "N"
		}
		l.Push(lua.LString(s))
		return 1
	})
	luaRegister(l, "posX", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.pos[0] - sys.cam.Pos[0]))
		return 1
	})
	luaRegister(l, "posY", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.pos[1]))
		return 1
	})
	luaRegister(l, "power", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.getPower()))
		return 1
	})
	luaRegister(l, "powermax", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.powerMax))
		return 1
	})
	luaRegister(l, "guardpoints", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.guardPoints))
		return 1
	})
	luaRegister(l, "guardpointsmax", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.guardPointsMax))
		return 1
	})
	luaRegister(l, "dizzypoints", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.dizzyPoints))
		return 1
	})
	luaRegister(l, "dizzypointsmax", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.dizzyPointsMax))
		return 1
	})
	luaRegister(l, "redlife", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.redLife))
		return 1
	})
	luaRegister(l, "playeridexist", func(*lua.LState) int {
		l.Push(lua.LBool(sys.playerIDExist(
			BytecodeInt(int32(numArg(l, 1)))).ToB()))
		return 1
	})
	luaRegister(l, "prevstateno", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.ss.prevno))
		return 1
	})
	luaRegister(l, "projcanceltime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.projCancelTime(
			BytecodeInt(int32(numArg(l, 1)))).ToI()))
		return 1
	})
	luaRegister(l, "projcontacttime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.projContactTime(
			BytecodeInt(int32(numArg(l, 1)))).ToI()))
		return 1
	})
	luaRegister(l, "projguardedtime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.projGuardedTime(
			BytecodeInt(int32(numArg(l, 1)))).ToI()))
		return 1
	})
	luaRegister(l, "projhittime", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.projHitTime(
			BytecodeInt(int32(numArg(l, 1)))).ToI()))
		return 1
	})
	luaRegister(l, "ratiolevel", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.ratioLevel()))
		return 1
	})
	luaRegister(l, "rightedge", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.rightEdge()))
		return 1
	})
	luaRegister(l, "roundno", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.round))
		return 1
	})
	luaRegister(l, "roundsexisted", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.roundsExisted()))
		return 1
	})
	luaRegister(l, "roundstate", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.roundState()))
		return 1
	})
	luaRegister(l, "screenheight", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.screenHeight()))
		return 1
	})
	luaRegister(l, "screenposX", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.screenPosX()))
		return 1
	})
	luaRegister(l, "screenposY", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.screenPosY()))
		return 1
	})
	luaRegister(l, "screenwidth", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.screenWidth()))
		return 1
	})
	luaRegister(l, "selfanimexist", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.selfAnimExist(
			BytecodeInt(int32(numArg(l, 1)))).ToB()))
		return 1
	})
	luaRegister(l, "stateno", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.ss.no))
		return 1
	})
	luaRegister(l, "stateOwner", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.ss.sb.playerNo + 1))
		return 1
	})
	luaRegister(l, "statetype", func(*lua.LState) int {
		var s string
		switch sys.debugWC.ss.stateType {
		case ST_S:
			s = "S"
		case ST_C:
			s = "C"
		case ST_A:
			s = "A"
		case ST_L:
			s = "L"
		}
		l.Push(lua.LString(s))
		return 1
	})
	luaRegister(l, "stagevar", func(*lua.LState) int {
		var s string
		switch strArg(l, 1) {
		case "info.name":
			s = sys.stage.name
		case "info.displayname":
			s = sys.stage.displayname
		case "info.author":
			s = sys.stage.author
		}
		l.Push(lua.LString(s))
		return 1
	})
	luaRegister(l, "sysfvar", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.sysFvarGet(int32(numArg(l, 1))).ToF()))
		return 1
	})
	luaRegister(l, "sysvar", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.sysVarGet(int32(numArg(l, 1))).ToI()))
		return 1
	})
	luaRegister(l, "teammode", func(*lua.LState) int {
		var s string
		switch sys.tmode[sys.debugWC.playerNo&1] {
		case TM_Single:
			s = "single"
		case TM_Simul:
			s = "simul"
		case TM_Turns:
			s = "turns"
		case TM_Tag:
			s = "tag"
		}
		l.Push(lua.LString(s))
		return 1
	})
	luaRegister(l, "teamside", func(*lua.LState) int {
		l.Push(lua.LNumber(int32(sys.debugWC.teamside) + 1))
		return 1
	})
	luaRegister(l, "tickspersecond", func(*lua.LState) int {
		l.Push(lua.LNumber(FPS))
		return 1
	})
	luaRegister(l, "time", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.ss.time))
		return 1
	})
	luaRegister(l, "topedge", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.topEdge()))
		return 1
	})
	luaRegister(l, "uniqhitcount", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.uniqHitCount))
		return 1
	})
	luaRegister(l, "var", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.varGet(int32(numArg(l, 1))).ToI()))
		return 1
	})
	luaRegister(l, "velX", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.vel[0]))
		return 1
	})
	luaRegister(l, "velY", func(*lua.LState) int {
		l.Push(lua.LNumber(sys.debugWC.vel[1]))
		return 1
	})
	luaRegister(l, "win", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.win()))
		return 1
	})
	luaRegister(l, "winko", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winKO()))
		return 1
	})
	luaRegister(l, "wintime", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winTime()))
		return 1
	})
	luaRegister(l, "winperfect", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winPerfect()))
		return 1
	})
	luaRegister(l, "winnormal", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winNormal()))
		return 1
	})
	luaRegister(l, "winspecial", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winSpecial()))
		return 1
	})
	luaRegister(l, "winhyper", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winHyper()))
		return 1
	})
	luaRegister(l, "wincheese", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winCheese()))
		return 1
	})
	luaRegister(l, "winthrow", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winThrow()))
		return 1
	})
	luaRegister(l, "winsuicide", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winSuicide()))
		return 1
	})
	luaRegister(l, "winteammate", func(*lua.LState) int {
		l.Push(lua.LBool(sys.debugWC.winTeammate()))
		return 1
	})
}

// Debug Script

func debugScriptInit(l *lua.LState, file string) error {
	scriptCommonInit(l)
	triggerScriptInit(l)
	luaRegister(l, "puts", func(*lua.LState) int {
		fmt.Println(strArg(l, 1))
		return 0
	})
	luaRegister(l, "setLife", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil && sys.debugWC.alive() {
			sys.debugWC.lifeSet(int32(numArg(l, 1)))
		}
		return 0
	})
	luaRegister(l, "setPower", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.debugWC.setPower(int32(numArg(l, 1)))
		}
		return 0
	})
	luaRegister(l, "setGuardPoints", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.debugWC.guardPointsSet(int32(numArg(l, 1)))
		}
		return 0
	})
	luaRegister(l, "setDizzyPoints", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.debugWC.dizzyPointsSet(int32(numArg(l, 1)))
		}
		return 0
	})
	luaRegister(l, "setRedLife", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.debugWC.redLifeSet(int32(numArg(l, 1)))
		}
		return 0
	})
	luaRegister(l, "removeDizzy", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.debugWC.unsetSCF(SCF_dizzy)
		}
		return 0
	})
	luaRegister(l, "selfState", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.debugWC.selfState(int32(numArg(l, 1)), -1, -1, 1)
		}
		return 0
	})
	luaRegister(l, "addHotkey", func(*lua.LState) int {
		l.Push(lua.LBool(func() bool {
			k := StringToKey(strArg(l, 1))
			if k == glfw.KeyUnknown {
				return false
			}
			sk := *NewShortcutKey(k, boolArg(l, 2), boolArg(l, 3), boolArg(l, 4))
			scr := strArg(l, 5)
			sys.shortcutScripts[sk] = &ShortcutScript{Script: scr}
			return true
		}()))
		return 1
	})
	luaRegister(l, "toggleClsnDraw", func(*lua.LState) int {
		sys.clsnDraw = !sys.clsnDraw
		return 0
	})
	luaRegister(l, "toggleDebugDraw", func(*lua.LState) int {
		sys.debugDraw = !sys.debugDraw
		return 0
	})
	luaRegister(l, "togglePause", func(*lua.LState) int {
		if sys.netInput == nil {
			sys.paused = !sys.paused
		}
		return 0
	})
	luaRegister(l, "step", func(*lua.LState) int {
		sys.step = true
		return 0
	})
	luaRegister(l, "toggleStatusDraw", func(*lua.LState) int {
		sys.statusDraw = !sys.statusDraw
		return 0
	})
	luaRegister(l, "toggleMaxPowerMode", func(*lua.LState) int {
		sys.maxPowerMode = !sys.maxPowerMode
		if sys.maxPowerMode {
			for _, c := range sys.chars {
				if len(c) > 0 {
					c[0].power = c[0].powerMax
				}
			}
		}
		return 0
	})
	luaRegister(l, "togglePlayer", func(*lua.LState) int {
		pn := int(numArg(l, 1))
		if pn < 1 || pn > len(sys.chars) || len(sys.chars[pn-1]) == 0 {
			return 0
		}
		for _, ch := range sys.chars[pn-1] {
			if ch.scf(SCF_disabled) {
				ch.unsetSCF(SCF_disabled)
			} else {
				ch.setSCF(SCF_disabled)
			}
		}
		return 0
	})
	luaRegister(l, "roundReset", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.roundResetFlg = true
		}
		return 0
	})
	luaRegister(l, "reload", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.reloadFlg = true
			for i := range sys.reloadCharSlot {
				sys.reloadCharSlot[i] = true
			}
		}
		return 0
	})
	luaRegister(l, "setAccel", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.accel = float32(numArg(l, 1))
		}
		return 0
	})
	luaRegister(l, "setAILevel", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			level := float32(numArg(l, 1))
			sys.com[sys.debugWC.playerNo] = level
			for _, c := range sys.chars[sys.debugWC.playerNo] {
				if level == 0 {
					c.key = sys.debugWC.playerNo
				} else {
					c.key = ^sys.debugWC.playerNo
				}
			}
		}
		return 0
	})
	luaRegister(l, "setTime", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			sys.time = int32(numArg(l, 1))
		}
		return 0
	})
	luaRegister(l, "clear", func(*lua.LState) int {
		for i := range sys.clipboardText {
			sys.clipboardText[i] = nil
		}
		return 0
	})
	luaRegister(l, "getAllowDebugKeys", func(*lua.LState) int {
		l.Push(lua.LBool(sys.allowDebugKeys))
		return 1
	})
	luaRegister(l, "resetScore", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			side := int(numArg(l, 1)) - 1
			for _, p := range sys.chars {
				if len(p) > 0 && p[0].teamside == side {
					p[0].scoreAdd(-p[0].scoreCurrent)
				}
			}
		}
		return 0
	})
	luaRegister(l, "markCheat", func(*lua.LState) int {
		if sys.netInput == nil && sys.fileInput == nil {
			side := int(numArg(l, 1)) - 1
			for _, p := range sys.chars {
				if len(p) > 0 && p[0].teamside == side {
					p[0].cheated = true
				}
			}
		}
		return 0
	})
	return l.DoFile(file)
}
