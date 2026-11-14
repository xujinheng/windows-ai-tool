package main

// Physical work-area coordinates may be negative for secondary monitors.
type desktopWorkArea struct { left,top,right,bottom int32 }

func desktopFitSize(w,h int32,work desktopWorkArea,margin int32)(int32,int32) {
    availableW,availableH:=work.right-work.left-2*margin,work.bottom-work.top-2*margin
    if availableW<1 {availableW=1};if availableH<1 {availableH=1}
    if w>availableW {w=availableW};if h>availableH {h=availableH}
    return w,h
}
func desktopClampPosition(x,y,w,h int32,work desktopWorkArea)(int32,int32){
    maxX,maxY:=work.right-w,work.bottom-h
    if maxX<work.left {maxX=work.left};if maxY<work.top {maxY=work.top}
    if x<work.left {x=work.left};if x>maxX {x=maxX}
    if y<work.top {y=work.top};if y>maxY {y=maxY}
    return x,y
}
func desktopPopupPlacement(anchorX,anchorY,w,h int32,work desktopWorkArea)(int32,int32,int32,int32){
    w,h=desktopFitSize(w,h,work,12)
    x,y:=desktopClampPosition(anchorX+18,anchorY+22,w,h,work)
    return x,y,w,h
}
