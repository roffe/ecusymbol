package symbol

var T5Types = map[string]uint8{
	"Batt_korr_tab!":    0x00,
	"D_fors_a!":         CHAR,
	"D_fors!":           CHAR,
	"Fuel_knock_mat!":   CHAR,
	"Fuel_knock_xaxis!": CHAR,
	"Fuel_map_xaxis!":   CHAR,
	"I_fors_a!":         CHAR,
	"I_fors!":           CHAR,
	"Ign_map_0!":        SIGNED,
	"Ign_map_1!":        SIGNED,
	"Inj_konst!":        CHAR,
	"Insp_mat!":         CHAR,
	"P_fors_a!":         CHAR,
	"P_fors!":           CHAR,
	"Pwm_ind_rpm!":      0x00,
	"Pwm_ind_trot!":     CHAR,
	"Reg_kon_mat!":      CHAR,
	"Reg_kon_mat_a!":    CHAR,
	"Reg_last!":         CHAR,
	"Regl_tryck_fgm!":   CHAR,
	"Regl_tryck_sgm!":   CHAR,
	"Regl_tryck_fgaut!": CHAR,
	"Tryck_mat_a!":      CHAR,
	"Tryck_mat!":        CHAR,
	"Knock_ref_matrix!": 0x00,
	"Knock_lim_tab!":    0x00,
	"Apc_knock_tab!":    CHAR,
	"I_kyl_st!":         SIGNED | CHAR,
	"Ign_map_1_y_axis!": SIGNED,
	"Idle_st_last!":     CHAR,
	"Idle_fuel_korr!":   CHAR,
	"Tryck_vakt_tab!":   CHAR,
	"Eftersta_fak!":     CHAR,
	"Eftersta_fak2!":    CHAR,
	"Temp_steg!":        CHAR,
	"Kyltemp_steg!":     CHAR,
	"Kyltemp_tab!":      CHAR,
}

var T5Offsets = map[string]float64{
	"Accel_konst!":          1,    // 128/256
	"Adapt_inj_imat!":       0.75, // 384/512
	"Adapt_injfaktor_high!": 0.75, // 384/512
	"Adapt_injfaktor_low!":  0.75, // 384/512
	"Adapt_injfaktor!":      0.75, // 384/512
	"Adapt_korr_high!":      0.75, // 384/512
	"Adapt_korr_low!":       0.75, // 384/512
	"Adapt_korr!":           0.75, // 384/512
	"Adapt_korr":            0.75, // 384/512
	"Adapt_ref!":            0.75, // 384/512
	"Adapt_ref":             0.75, // 384/512
	"After_fcut_tab!":       1,
	"Cyl_komp!":             0.75, // 384/512
	"Del_mat!":              0,
	"Detect_map_x_axis!":    -1,
	"Diag_speed_load!":      -1,
	"Eftersta_fak!":         1,
	"Eftersta_fak2!":        1,
	"Fload_tab!":            1,
	"Fuel_knock_mat!":       0.5, // 128/256
	"Fuel_knock_xaxis!":     -1,
	"Fuel_map_xaxis!":       -1,
	"Grund_last!":           -1,
	"Hot_start_fak!":        1, // 128/256
	"Hot_tab!":              1,
	"Idle_fuel_korr!":       0.5, // 128/256
	"Idle_st_last!":         -1,
	"Idle_tryck!":           -1,
	"Ign_map_0_x_axis!":     -1,
	"Ign_map_0!":            0,
	"Ign_map_2_x_axis!":     -1,
	"Ign_map_4!":            0,
	"Ign_map_6_x_axis!":     -1,
	"Ign_map_7_x_axis!":     -1,
	"Inj_map_0!":            0,   // LOLA specific
	"Insp_mat!":             0.5, // 128/256
	"Iv_min_load!":          -1,
	"Kadapt_load_high!":     -1,
	"Kadapt_load_low!":      -1,
	"Knock_press_lim":       -1, // bar
	"Knock_press_tab!":      -1,
	"Knock_press!":          -1,
	"Lacc_konst!":           1, // 256/256
	"Lam_laststeg!":         -1,
	"Lam_minlast!":          -1,
	"Lambdaint!":            0.75, // 1/512
	"Limp_tryck_konst!":     -1,
	"Lret_konst!":           1, // 256/256
	"Luft_kompfak!":         0.75,
	"Max_ratio_aut!":        -1,
	"Max_regl_temp_1!":      -1,
	"Max_regl_temp_2!":      -1,
	"Misfire_map_x_axis!":   -1,
	"Open_loop_knock":       -1,
	"Open_loop":             -1,
	"P_Manifold":            -1,
	"Pressure map (AUT) scaled for 3 bar mapsensor": -1,
	"Pressure map scaled for 3 bar mapsensor":       -1,
	"Purge_map_xaxis!":                              -1,
	"Reg_last!":                                     0,
	"Regl_tryck_fgaut!":                             -1,
	"Regl_tryck_fgm!":                               -1,
	"Regl_tryck_sgm!":                               -1,
	"Regl_tryck":                                    -1,
	"Ret_fuel_fak!":                                 1, // 128/256
	"Ret_fuel_tab!":                                 1, // 128/256
	"Retard_konst!":                                 1, // 128/256
	"Shift_load!":                                   -1,
	"Shift_up_load_hyst!":                           -1,
	"Sond_heat_tab":                                 -1,
	"Tryck_mat_a!":                                  -1,
	"Tryck_mat!":                                    -1,
	"Tryck_vakt_tab!":                               -1,
	"Turbo_knock_press":                             -1, // bar
	"Turbo_knock_tab":                               -1,
	//"Temp_steg!":        -40,
}
