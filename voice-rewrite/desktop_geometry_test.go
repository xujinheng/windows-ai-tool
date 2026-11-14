package main

import "testing"

func TestDesktopPopupPlacementNegativeMonitor(t *testing.T) {
    secondary:=desktopWorkArea{-1920,-120,0,960}
    x,y,w,h:=desktopPopupPlacement(-15,910,576,620,secondary)
    if x!=-576 || y!=340 || w!=576 || h!=620 {t.Fatalf("secondary %d,%d %dx%d",x,y,w,h)}
    x,y,w,h=desktopPopupPlacement(-1910,-100,2200,1500,secondary)
    if x!=-1896 || y!=-96 || w!=1896 || h!=1056 {t.Fatalf("oversize %d,%d %dx%d",x,y,w,h)}
}
func TestDesktopFitSizeSmallWorkArea(t *testing.T) {
    work:=desktopWorkArea{0,0,1024,728}
    for _,s:=range []struct{w,h,ew,eh int32}{{714,720,714,704},{1428,1440,1000,704},{1071,1080,1000,704}}{
        w,h:=desktopFitSize(s.w,s.h,work,12)
        if w!=s.ew || h!=s.eh {t.Errorf("%v got %dx%d",s,w,h)}
    }
}
