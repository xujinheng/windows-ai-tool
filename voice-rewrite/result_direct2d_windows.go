//go:build windows

package main

import (
    "fmt"
    "math"
    "runtime"
    "syscall"
    "unsafe"
)

// resultD2D renders the actual AI result window with Windows Direct2D and
// DirectWrite. Real HWND EDIT/BUTTON controls remain responsible for selecting,
// copying, keyboard navigation and guarded application into the source app.
// This never loads a web renderer, Node.js or any third-party package.
type resultGUID struct {
    Data1 uint32
    Data2, Data3 uint16
    Data4 [8]byte
}
var (
    resultD2DIID = resultGUID{0x06152247, 0x6f50, 0x465a, [8]byte{0x92,0x45,0x11,0x8b,0xfd,0x3b,0x60,0x07}}
    resultDWIID  = resultGUID{0xb859ee5a,0xd838,0x4b5b,[8]byte{0xa2,0xe8,0x1a,0xdc,0x7d,0x93,0xdb,0x48}}
    resultD2DDLL = syscall.NewLazyDLL("d2d1.dll")
    resultDWDLL = syscall.NewLazyDLL("dwrite.dll")
    resultD2DCreate = resultD2DDLL.NewProc("D2D1CreateFactory")
    resultDWCreate = resultDWDLL.NewProc("DWriteCreateFactory")
)
type resultD2DColor struct{ R,G,B,A float32 }
type resultD2DRect struct{ Left,Top,Right,Bottom float32 }
type resultD2DRound struct {
    Rect resultD2DRect
    RX,RY float32
}
type resultD2DMatrix struct{ M11,M12,M21,M22,DX,DY float32 }
type resultD2DProps struct {
    Kind,PixelFormat,AlphaMode uint32
    DpiX,DpiY float32
    Usage,MinLevel uint32
}
type resultD2DHWNDProps struct {
    HWND uintptr
    Width,Height,Present,Reserved uint32
}
type resultRenderer struct {
    hwnd uintptr
    scale float32
    factory,writeFactory,target uintptr
    brushes [9]uintptr
    formats [4]uintptr
    colors [9]uint32
    title string
    allowApply bool
    dark bool
}
func resultCOM(obj uintptr, slot int, args ...uintptr) uintptr {
    if obj==0{return 0x80004003}
    vt:=*(*uintptr)(unsafe.Pointer(obj))
    fn:=*(*uintptr)(unsafe.Pointer(vt+uintptr(slot)*unsafe.Sizeof(uintptr(0))))
    call:=make([]uintptr,0,len(args)+1)
    call=append(call,obj)
    call=append(call,args...)
    hr,_,_:=syscall.SyscallN(fn,call...)
    return hr
}
func resultRelease(obj uintptr){ if obj!=0 {resultCOM(obj,2)} }
func resultFloat(v float32) uintptr{return uintptr(math.Float32bits(v))}
func resultColor(c uint32) resultD2DColor {
    return resultD2DColor{float32((c>>16)&255)/255,float32((c>>8)&255)/255,float32(c&255)/255,1}
}
func (d *resultRenderer) palette(dark bool) {
    d.dark=dark
    if dark {
        d.colors=[9]uint32{0x101218,0x181b23,0x232631,0x2b2e3a,0xf2f1f7,0xa4a5b7,0x353948,0xbaaeff,0xdcd7ff}
    } else {
        d.colors=[9]uint32{0xf8f9fc,0xf0f1f6,0xffffff,0xf8f9fc,0x252637,0x777b8c,0xe5e6ef,0x7258cf,0xeeeafd}
    }
}
func newResultRenderer(hwnd uintptr, scale float64, title string, dark, allowApply bool) (*resultRenderer,error) {
    d:=&resultRenderer{hwnd:hwnd,scale:float32(scale),title:title,allowApply:allowApply}
    d.palette(dark)
    if hr,_,_:=resultD2DCreate.Call(0,uintptr(unsafe.Pointer(&resultD2DIID)),0,uintptr(unsafe.Pointer(&d.factory)));int32(hr)<0 {
        return nil,fmt.Errorf("Direct2D factory: 0x%x",uint32(hr))
    }
    if hr,_,_:=resultDWCreate.Call(0,uintptr(unsafe.Pointer(&resultDWIID)),uintptr(unsafe.Pointer(&d.writeFactory)));int32(hr)<0 {
        d.close()
        return nil,fmt.Errorf("DirectWrite factory: 0x%x",uint32(hr))
    }
    // Three compact typographic levels + a monogram. Text is rendered by
    // DirectWrite with UTF-16 code-unit lengths, including Chinese glyphs.
    weights:=[4]uint32{400,500,600,700}
    sizes:=[4]float32{12,15,25,17}
    for i:=range d.formats {
        family,_:=syscall.UTF16PtrFromString("Microsoft YaHei UI")
        locale,_:=syscall.UTF16PtrFromString("zh-cn")
        hr:=resultCOM(d.writeFactory,15,uintptr(unsafe.Pointer(family)),0,uintptr(weights[i]),0,5,resultFloat(sizes[i]),uintptr(unsafe.Pointer(locale)),uintptr(unsafe.Pointer(&d.formats[i])))
        runtime.KeepAlive(family);runtime.KeepAlive(locale)
        if int32(hr)<0 {
            d.close()
            return nil,fmt.Errorf("DirectWrite format %d: 0x%x",i,uint32(hr))
        }
    }
    if err:=d.createTarget();err!=nil {d.close();return nil,err}
    return d,nil
}
func (d *resultRenderer) disposeTarget() {
    for i:=range d.brushes {resultRelease(d.brushes[i]);d.brushes[i]=0}
    resultRelease(d.target);d.target=0
}
func (d *resultRenderer) close() {
    if d==nil{return}
    d.disposeTarget()
    for i:=range d.formats {resultRelease(d.formats[i]);d.formats[i]=0}
    resultRelease(d.writeFactory);d.writeFactory=0
    resultRelease(d.factory);d.factory=0
}
func (d *resultRenderer) createTarget() error {
    var rc rect32
    procGetClientRect.Call(d.hwnd,uintptr(unsafe.Pointer(&rc)))
    width,height:=rc.Right-rc.Left,rc.Bottom-rc.Top
    if width<1 || height<1{return fmt.Errorf("empty Direct2D result surface")}
    props:=resultD2DProps{DpiX:96,DpiY:96}
    hwndProps:=resultD2DHWNDProps{HWND:d.hwnd,Width:uint32(width),Height:uint32(height)}
    hr:=resultCOM(d.factory,14,uintptr(unsafe.Pointer(&props)),uintptr(unsafe.Pointer(&hwndProps)),uintptr(unsafe.Pointer(&d.target)))
    if int32(hr)<0{return fmt.Errorf("Direct2D HWND target: 0x%x",uint32(hr))}
    for i,c:=range d.colors {
        color:=resultColor(c)
        hr=resultCOM(d.target,8,uintptr(unsafe.Pointer(&color)),0,uintptr(unsafe.Pointer(&d.brushes[i])))
        if int32(hr)<0 {
            d.disposeTarget()
            return fmt.Errorf("Direct2D brush %d: 0x%x",i,uint32(hr))
        }
    }
    return nil
}
func (d *resultRenderer) updateColors(dark bool) {
    if d.dark==dark{return}
    d.palette(dark)
    // Recreating the target ensures colors are refreshed atomically.
    d.disposeTarget()
}
func (d *resultRenderer) resized(){
    if d==nil || d.target==0{return}
    var rc rect32
    procGetClientRect.Call(d.hwnd,uintptr(unsafe.Pointer(&rc)))
    w,h:=rc.Right-rc.Left,rc.Bottom-rc.Top
    if w<1 || h<1{return}
    size:=[2]uint32{uint32(w),uint32(h)}
    if hr:=resultCOM(d.target,58,uintptr(unsafe.Pointer(&size[0])));int32(hr)<0{d.disposeTarget()}
}
func (d *resultRenderer) round(x,y,w,h,r float32,brush int){
    shape:=resultD2DRound{resultD2DRect{x,y,x+w,y+h},r,r}
    resultCOM(d.target,19,uintptr(unsafe.Pointer(&shape)),d.brushes[brush])
}
func (d *resultRenderer) outline(x,y,w,h,r float32,brush int){
    shape:=resultD2DRound{resultD2DRect{x,y,x+w,y+h},r,r}
    resultCOM(d.target,18,uintptr(unsafe.Pointer(&shape)),d.brushes[brush],resultFloat(1),0)
}
func (d *resultRenderer) fill(x,y,w,h float32,brush int){
    area:=resultD2DRect{x,y,x+w,y+h}
    resultCOM(d.target,17,uintptr(unsafe.Pointer(&area)),d.brushes[brush])
}
func (d *resultRenderer) text(t string,x,y,w,h float32,format,brush int){
    if t=="" || format<0 || format>=len(d.formats){return}
    utf:=syscall.StringToUTF16(t)
    area:=resultD2DRect{x,y,x+w,y+h}
    resultCOM(d.target,27,uintptr(unsafe.Pointer(&utf[0])),uintptr(len(utf)-1),d.formats[format],uintptr(unsafe.Pointer(&area)),d.brushes[brush],0,0)
    runtime.KeepAlive(utf)
}
// Draw only decorative primitives and visual headings; native controls are not
// overpainted and retain actual focus, text selection, IME and accessibility.
func (d *resultRenderer) draw() bool {
    if d==nil{return false}
    if d.target==0 {
        if err:=d.createTarget();err!=nil{return false}
    }
    var rc rect32
    procGetClientRect.Call(d.hwnd,uintptr(unsafe.Pointer(&rc)))
    w:=float32(rc.Right)/d.scale
    h:=float32(rc.Bottom)/d.scale
    transform:=resultD2DMatrix{M11:d.scale,M22:d.scale}
    resultCOM(d.target,30,uintptr(unsafe.Pointer(&transform)))
    resultCOM(d.target,48) // BeginDraw
    bg:=resultColor(d.colors[0])
    resultCOM(d.target,47,uintptr(unsafe.Pointer(&bg))) // Clear
    // A quiet utility header, not a dashboard. Brand is the sole accent.
    d.round(27,24,34,34,11,7)
    d.text("V",38,30,20,25,3,2)
    d.text("VoiceRewrite",75,24,w-170,27,1,4)
    d.text("智能写作 · 本地结果",75,49,w-170,19,0,5)
    // Text-first hierarchy: action title followed by preserved source status.
    d.text(d.title,30,91,w-60,41,2,4)
    // Reading surface and one thin divider replace multiple stacked frames.
    d.round(23,164,w-46,h-253,16,2)
    d.outline(23,164,w-46,h-253,16,6)
    d.fill(28,h-70,w-56,1,6)
    d.text("可选择正文并复制  ·  Esc 关闭",31,h-52,w-290,22,0,5)
    hr:=resultCOM(d.target,49,0,0) // EndDraw
    if uint32(hr)==0x8899000c { // D2DERR_RECREATE_TARGET
        d.disposeTarget()
    }
    return int32(hr)>=0
}
