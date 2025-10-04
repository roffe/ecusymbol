package symbol

var T5Types = map[string]uint8{
	"Ign_map_0!":      SIGNED,
	"Inj_konst!":      CHAR,
	"Insp_mat!":       CHAR,
	"Fuel_map_xaxis!": CHAR,
	"Batt_korr_tab!":  0x00,
	"Tryck_mat!":      CHAR,
	"Pwm_ind_trot!":   CHAR,
	"Pwm_ind_rpm!":    0x00,
}

var T5Offsets = map[string]float64{
	"Insp_mat!":         0.5,
	"Fuel_map_xaxis!":   -1,
	"Tryck_mat!":        -1,
	"Ign_map_0_x_axis!": -1,
}
