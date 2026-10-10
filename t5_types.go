package symbol

import "strings"

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
	"Regl_tryck_fga!":   CHAR,
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
	"Lufttemp_steg!":    CHAR,
	"Luft_kompfak!":     CHAR,

	// Word tables that t5SixteenBit doesn't list. Knock_ref_rpm! is an axis
	// T5Suite reads as 16-bit in its axis code; the rest are newer symbols
	// (or 4-byte values) whose bins hold words, e.g. 03 E8 = 1000.
	"AC_Control!":            0x00,
	"AdjustLamCal!":          0x00,
	"Cat_load_tab!":          0x00,
	"Cat_ox1_filt_coef_map!": 0x00,
	"Cat_ox2_dev_max_map!":   0x00,
	"Cat_ox2_dev_max_tab!":   0x00,
	"Cat_ox2_filt_coef_map!": 0x00,
	"CwIgnCal!":              0x00,
	"Fsond_lamtime!":         0x00,
	"Idle_ac_extra_sp!":      0x00,
	"Idle_ac_extra_tab!":     0x00,
	"Ign_counter_mis_lim!":   0x00,
	"Ign_map_request!":       0x00,
	"Ign_map_value!":         0x00,
	"Knock_ref_rpm!":         0x00,
	"LhomeYaxisPman!":        0x00,
	"Misf_ign_lim!":          0x00,
	"P_man_korr_axis!":       0x00,
	"PMCal_OpenPWMTab!":      0x00,
	"PMCal_OpenTimeTab!":     0x00,
	"Press_trans_type!":      0x00,
	"Purge_map_yaxis!":       0x00,
	"T1_max!":                0x00,
	"T2_max!":                0x00,
	"Tank_start_time!":       0x00,
	"Weight!":                0x00,
}

// t5SixteenBit reports whether T5Suite treats the table name as 16-bit
// (Trionic5FileInformation.isSixteenBitTable); every other table is bytes.
// Reg_kon_mat is decided by length in tosymbol.
func t5SixteenBit(name string) bool {
	if strings.HasPrefix(name, "Ign_angle_byte") {
		return false
	}
	for _, p := range t5SixteenBitPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

var t5SixteenBitPrefixes = []string{
	"Ign_map_0", "Ign_map_1", "Ign_map_2", "Ign_map_3", "Ign_map_4", "Ign_map_5", "Ign_map_6",
	"Ign_map_7", "Ign_map_8", "Inj_map_0", "Idle_step_ioka!", "Idle_step_imns!", "Derivata_grans!",
	"Lambdamatris!", "Lambdamatris_diag!", "Iv_min_tab!", "Iv_min_tab_ac!", "Misfire_map_x_axis!",
	"Max_ratio_aut!", "Mis200_map!", "Misfire_map!", "Start_insp!", "Start_max!", "Start_ramp!",
	"Max_regl_temp_1", "Max_regl_temp_2", "Mis1000_map!", "Knock_ref_matrix!", "Detect_map!",
	"Knock_wind_on_tab!", "Lknock_oref_tab!", "Knock_lim_tab!", "Knock_wind_off_tab!",
	"Turbo_knock_tab!", "Knock_press_tab!", "Idle_drift_tab!", "Last_reg_kon!",
	"Shift_rpm! (automaat?)", "Ign_map_6_x_axis!", "Ign_map_6_y_axis!", "Gear_st!", "Reg_varv!",
	"Trans_y_st!", "Idle_st_rpm!", "Dash_rpm_axis!", "Ramp_down_ram!", "Ramp_up_ram!",
	"Idle_start_extra!", "Idle_start_extra_ramp!", "Idle_temp_off!", "Idle_rpm_tab!",
	"Derivata_br_sp!", "Derivata_br_tab_pos!", "Derivata_br_tab_neg!", "Br_plus_tab!",
	"Br_minus_tab!", "Start_tab!", "Temp_reduce_y_st!", "Idle_tryck!", "Iv_start_time_tab!",
	"Cat_ox1_filt_coef_tab!", "Cat_ox2_filt_coef_tab!", "Cat_air_flow_tab!", "Cat_ox1_per_hi_tab!",
	"Cat_ox1_per_lo_tab!", "Cat_ox1_err_max_tab!", "Cat_ox1_dev_max_tab!", "Batt_korr_tab!",
	"Kadapt_max_ref!", "Ign_map_3_x_axis!", "Last_varv_st!", "Fuel_map_yaxis!", "Ign_map_3_y_axis!",
	"Pwm_ind_rpm!", "Max_regl_sp!", "Idle_ac_tab!", "Wait_count_tab!", "Knock_average_tab!",
	"Knock_wind_rpm!", "Detect_map_y_axis!", "Misfire_map_y_axis!", "Detect_map_x_axis!", "Rpm_max!",
	"Tid_Konst!", "Min_tid!", "Kadapt_rpm_high!", "Kadapt_rpm_low!", "Knock_start!",
	"Min_rpm_closed_loop!", "Min_rpm_gadapt!", "Ign_offset!", "Ign_offset_cyl1!", "Ign_offset_cyl2!",
	"Ign_offset_cyl3!", "Ign_offset_cyl4!", "Ign_offset_adapt!", "Max_rpm_gadapt!",
	"Temp_ramp_value!", "Global_adapt_nr!", "Ign_idle_angle!", "Knock_ang_dec", "Knock_wind_rpm",
	"Knock_matrix_time", "Radius_of_roll!", "Knock_ref_tab!", "Frek_230!", "Frek_250!",
	"Mis_trans_limit!", "Derivata_fuel_rpm!", "Tid_konst!", "Ret_delta_rpm!", "Ret_down_rpm!",
	"Ret_up_rpm!", "Start_time_rpm_lim!", "Start_v_vinkel!", "Cut_ej_under!", "Open_all_varv!",
	"Open_varv!", "Vinkel_konst!", "API_ku_delay!", "API_ku_derivata!", "API_ku_offset!",
	"API_ku_ramp!", "Airpump_ign_offset!", "Rev_grad_sens_cont!", "PMCal_RpmIdleNomRefLim!",
	"Ap_max_on_time!", "Ap_max_rpm!", "API_dt_delay!", "API_dt_ramp!", "API_rpm_limit!",
	"Diag_speed_time!", "Ox2_activity_time_lim!", "Ox2_change_lim!", "PMCal_CloseRamp!",
	"PMCal_IdlePosFiltCoef!", "PMCal_IdlePosIntABSLim!", "PMCal_IdleRefHaltLim!",
	"PMCal_IdleValueLim!", "PMCal_StartRamp!", "Shut_off_time!", "Sond_omsl_lim!", "Start_detekt_nr!",
	"Start_detekt_rpm!", "Synk_ok_lim!", "Ap_lambda_delay!", "Cat_af_start_timer_min!",
	"Cat_air_flow_hi!", "Cat_air_flow_lo!", "Cat_load_filt_coef!", "Cat_ox1_bias_lim!",
	"Cat_ox1_err_lim!", "Cat_ox2_filt_coef_2!", "Cat_stage1_threshold!", "Cat_start_timer_lim!",
	"Lambda_cat_lean!", "Lambda_cat_rich!", "PMCal_LambdaAvgHaltLim!", "PMCal_LambdaAvgLim!",
	"Sond_ign_limit!", "AC_delay!", "Adapt_purge_period!", "Ap_ign_offset_delay!",
	"Ap_ign_offset_delay2!", "Ap_ign_step!", "Ap_inj_factor_delay!", "Ap_inj_factor_delay2!",
	"Ap_max_on_time2!", "Ap_min_stop_time!", "Ap_start_delay!", "Ap_start_delay2!",
	"Cat_fc_timer_lim!", "Cat_hl_air_flow_min!", "Cat_hl_timer_lim!", "Cat_vs_timer_lim!",
	"Cl_timer1_lim!", "Cl_timer2_lim!", "Comb_w_temp_limbhome!", "I_fak_max!",
	"Ign_idle_angle_start!", "Jd_lastvar!", "Kadapt_period!", "Knock_back!", "Knock_offset!",
	"Knock_reduce!", "LLS_min!", "Mainrly_off!", "Max_neg_der!", "Max_pos_der!", "Max_um_time!",
	"Nr_adapt_idle_mat!", "Misf_rpm_diff!", "Misf_rpm_diff_start!", "P_tank_filt_fak!", "Pdec_min!",
	"Pinc_min!", "Pperf_min!", "Press_perf_max!", "Press_perf_min!", "Press_rpm_lim!",
	"Rev_grad_sens_dur_req!", "Rpm_dif!", "Rpm_perf_max!", "Rpm_perf_min!", "Shift_max_rpm!",
	"Shift_up_off_time!", "Shift_up_rpm_hyst!", "Shift_up_time!", "Sync_counter!", "Sync_rpm!",
	"Trans_offset!", "Trans_trott_limit!", "Diag_speed_rpm!", "I_last_rpm!", "Cat_rpm_tab!",
	"Lam_rpm_sp!", "Shift_rpm!", "Pulses_per_rev!", "Apc_adapt", "Knock_count_cyl", "Knock_count_map",
	"Adapt_ggr", "Knock_average", "Knock_average_limit", "Knock_map_lim", "Knock_map_offset",
	"Knock_offset1234", "Lknock_oref_level", "Turbo_knock_press", "Knock_offset1", "Knock_offset2",
	"Knock_offset3", "Knock_offset4", "Knock_press_limit", "Knock_diag_level", "Knock_level",
	"Knock_lim", "Knock_ref_level", "Knock_wind_off", "Knock_wind_on", "Knock_wind_off_ang",
	"Knock_wind_on_ang", "Ign_angle", "Reg_kon_apc",
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
	"Regl_tryck_fga!":                               -1,
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
