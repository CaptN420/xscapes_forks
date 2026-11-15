package companion

import (
	"fmt"
	"testing"
)

func TestDebugPrintPoses(t *testing.T) {
	for _, clip := range []string{ClipIdle, ClipJoy, ClipSad, ClipSleep, ClipEat, ClipPlay, ClipPet, ClipSick} {
		v := NewTamagotchiState()
		st := &v
		r := NewTamagotchiRenderer(st)
		st.Animation = clip
		st.AnimLeft = 0
		st.Sleeping = clip == ClipSleep
		tm := 0.0
		for i := 0; i < 30; i++ { // settle
			tm += 1.0 / 12
			tamRegion(r, tm)
		}
		fmt.Printf("=== %s ===\n", clip)
		g := tamRegion(r, tm+1.0/12)
		for y := 0; y < 14; y++ {
			fmt.Println(string(g[y*18 : y*18+18]))
		}
	}
}
