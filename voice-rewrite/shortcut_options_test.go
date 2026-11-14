package main

import "testing"

func TestVoiceShortcutNormalization(t *testing.T) {
    tests := []struct{raw,want string; ok bool}{
        {"Mouse4","Mouse4",true},{"xbutton2","Mouse5",true},
        {"f8","F8",true},{"F24","F24",true},
        {"Ctrl+p","Ctrl+P",true},{"ctrl+alt+shift+f8","Ctrl+Alt+Shift+F8",true},
        {"Alt+PageDown","Alt+PageDown",true},
        {"f0","",false},{"F25","",false},{"A","",false},{"Ctrl","",false},
        {"Shift+A","",false},{"Win+P","",false},{"Ctrl+Ctrl+P","",false},
    }
    for _,tc:=range tests {
        got,_,_,_,err:=parseShortcutParts(tc.raw)
        if (err==nil)!=tc.ok || (tc.ok && got!=tc.want) {
            t.Errorf("input=%q got=%q err=%v want=%q",tc.raw,got,err,tc.want)
        }
    }
}
func TestDictationAndRewriteShortcutCollisions(t *testing.T) {
    if _,err:=uxDictationShortcut("F9","f9","Ctrl+Alt+F10");err==nil { t.Fatal("dictation/rewrite collision accepted") }
    if _,err:=uxRewriteShortcut("ctrl+alt+f10","Mouse4","Ctrl+Alt+F10");err==nil { t.Fatal("exit collision accepted") }
    if v,err:=uxDictationShortcut("Ctrl+P","Mouse5","Ctrl+Alt+F10");err!=nil||v!="Ctrl+P" { t.Fatalf("combo: %q %v",v,err) }
    if v,err:=uxRewriteShortcut("Mouse5","Ctrl+P","Ctrl+Alt+F10");err!=nil||v!="Mouse5" { t.Fatalf("mouse: %q %v",v,err) }
    if _,err:=uxDictationShortcut("Pause","Mouse5","Ctrl+Alt+F10");err==nil { t.Fatal("Pause must not be a hold key") }
}
