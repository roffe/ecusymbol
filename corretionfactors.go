package symbol

var correctionFactors = map[string]float64{
	"AdpFuelProt.MulFuelAdapt":  0.01,
	"KnkSoundRedCal.fi_OffsMa":  0.1,
	"IgnE85Cal.fi_AbsMap":       0.1,
	"MAFCal.cd_ThrottleMap":     0.0009765625,
	"TrqMastCal.Trq_NominalMap": 0.1,
	"TrqMastCal.Trq_MBTMAP":     0.1,
	"AfterStCal.StartMAP":       0.0009765625, // 1/1024
	// "KnkFuelCal.EnrichmentMap":     0.0009765625, // 1/1024 // T8
	"KnkFuelCal.EnrichmentMap":     0.001,        // T7
	"AfterStCal.HotSoakMAP":        0.0009765625, // 1/1024
	"MAFCal.NormAdjustFacMap":      0.0078125,    // 1/128
	"BFuelCal.LambdaOneFacMap":     0.0078125,    // 1/128
	"BFuelCal.TempEnrichFacMap":    0.0078125,    // 1/128
	"BFuelCal.E85TempEnrichFacMap": 0.0078125,    // 1/128
	"AfterStCal.AmbientMAP":        0.0078125,    // 1/128
	"FFFuelCal.KnkEnrichmentMAP":   0.0078125,    // 1/128
	"FFFuelCal.TempEnrichFacMAP":   0.0078125,    // 1/128

	"E85.X_EthAct_Tech2": 0.1,

	"ActualIn.p_AirBefThrottle":         0.001,
	"ActualIn.p_AirInlet":               0.001,
	"AirCompCal.PressMap":               0.001,
	"BFuelCal.Map":                      0.01,
	"BFuelCal.StartMap":                 0.01,
	"BoostCal.RegMap":                   0.1,
	"DisplProt.LambdaScanner":           0.01,
	"LambdaScan.LambdaScanner":          0.01,
	"ECMStat.p_Diff":                    0.001,
	"ECMStat.p_DiffThrot":               0.001,
	"IgnAbsCal.fi_NormalMAP":            0.1,
	"IgnAbsCal.fi_lowOctanMAP":          0.1,
	"IgnAbsCal.fi_highOctanMAP":         0.1,
	"IgnIdleCal.fi_IdleMap":             0.1,
	"IgnMastProt.fi_Offset":             0.1,
	"IgnNormCal.Map":                    0.1,
	"IgnProt.fi_Offset":                 0.1,
	"IgnStartCal.fi_StartMap":           0.1,
	"IgnStartCal.X_EthActSP":            0.1,
	"In.p_AirAmbient":                   0.1,
	"In.p_AirBefThrottle":               0.001,
	"In.p_AirInlet":                     0.001,
	"In.v_Vehicle":                      0.1,
	"Lambda.LambdaInt":                  0.01,
	"MyrtilosCal.Launch_RPM":            100,
	"Out.fi_Ignition":                   0.1,
	"Out.PWM_BoostCntrl":                0.1,
	"Out.X_AccPedal":                    0.1,
	"Out.X_AccPos":                      0.1,
	"BstKnkCal.OffsetXSP":               0.1,
	"InjCorrCal.BattCorrSP":             0.1,
	"MAFCal.LoadYSP":                    0.001,
	"TrqLimCal.Trq_ManGear":             0.1,
	"TrqLimCal.Trq_MaxEngineTab1":       0.1,
	"TrqLimCal.Trq_MaxEngineTab2":       0.1,
	"FFTrqCal.FFTrq_MaxEngineTab1":      0.1,
	"FFTrqCal.FFTrq_MaxEngineTab2":      0.1,
	"MyrtilosAdap.WBLambda_FeedbackMap": 0.001,
	"MyrtilosAdap.WBLambda_FFMap":       1,
	"Myrtilos.InjectorDutyCycle":        1,
	"PedalMapCal.X_PedalMap":            0.1,
	//T5

	// "Reg_kon_mat"))
	// {
	// 	if (GetSymbolLength(symbolname) == 0x80)
	// 	{
	// 		returnvalue = 1;
	// 	}
	// 	else
	// 	{
	// 		returnvalue = 0.1;
	// 	}
	// }
	"Accel_konst!":            0.00390625,   //returnvalue = 0.0078125, // 1/12,
	"Adapt_inj_imat!":         0.001953125,  // 1/51,
	"Adapt_injfaktor_high!":   0.001953125,  // 1/51,
	"Adapt_injfaktor_low!":    0.001953125,  // 1/51,
	"Adapt_injfaktor!":        0.001953125,  // 1/51,
	"Adapt_korr_high!":        0.001953125,  // 1/51,
	"Adapt_korr_low!":         0.001953125,  // 1/51,
	"Adapt_korr!":             0.001953125,  // 1/512
	"Adapt_ref!":              0.001953125,  // 1/51,
	"Adapt_ref":               0.001953125,  // 1/51,
	"After_fcut_tab!":         0.0009765625, // 1/102,
	"Ap_max_rpm!":             10,
	"Apc_knock_tab!":          0.01,
	"Batt_korr_tab!":          0.004,       // 1/25,
	"Cyl_komp!":               0.001953125, // 1/512 //Cylinder Compensation: (Cyl_komp+384)/51,
	"Dash_rpm_axis!":          10,
	"Del_mat!":                3,
	"Derivata_fuel_rpm!":      10,
	"Derivata_grans!":         10,
	"Detect_map_x_axis!":      0.01,
	"Diag_speed_load!":        0.01,
	"Diag_speed_rpm!":         10,
	"Eftersta_fak!":           0.0078125,   // 0.01,
	"Eftersta_fak2!":          0.0078125,   //0.01,
	"Fload_tab!":              0.001953125, // 1/51,
	"Fuel_knock_mat!":         0.00390625,  // 1/25,
	"Fuel_map_xaxis!":         0.01,
	"Fuel_map_yaxis!":         10,
	"Gear_st!":                0.1, // 1/ ((256*256) / 260,
	"Grund_last!":             0.01,
	"Hot_start_fak!":          0.0009765625, // 128/25,
	"Hot_tab!":                0.0009765625, // 1/102,
	"Idle_fuel_korr!":         0.00390625,   // 1/25,
	"Idle_rpm_tab!":           10,
	"Idle_st_last!":           0.01,
	"Idle_st_rpm!":            10,
	"Idle_tryck!":             0.01,
	"Ign_idle_angle_start":    0.1,
	"Ign_idle_angle!":         0.1,
	"Ign_map_0_x_axis!":       0.01,
	"Ign_map_0!":              0.1,
	"Ign_map_1!":              0.1,
	"Ign_map_2_x_axis!":       0.01,
	"Ign_map_2!":              0.1,
	"Ign_map_3!":              0.1,
	"Ign_map_4!":              0.1,
	"Ign_map_5!":              0.1,
	"Ign_map_6_x_axis!":       0.01,
	"Ign_map_6!":              0.1,
	"Ign_map_7_x_axis!":       0.01,
	"Ign_map_7!":              0.1,
	"Ign_map_8!":              0.1,
	"Inj_map_0!":              1,          // 0.00390625, // 1/256 LOLA specifi
	"Insp_mat!":               0.00390625, // 1/256
	"Iv_min_load!":            0.01,
	"Kadapt_load_high!":       0.01,
	"Kadapt_load_low!":        0.01,
	"Kadapt_rpm_high!":        10,
	"Kadapt_rpm_low!":         10,
	"Knock_ang_dec!":          0.1,
	"Knock_average_tab!":      0.1,
	"Knock_average":           0.1,
	"Knock_lim_tab!":          0.1,
	"Knock_lim":               0.1,
	"Knock_press_lim":         0.01, // ba,
	"Knock_press_tab!":        0.01,
	"Knock_press!":            0.01,
	"Knock_wind_rpm!":         10,
	"Lacc_konst!":             0.00390625, // 1/256 //0.0078125, // 1/12,
	"Lam_laststeg!":           0.01,
	"Lam_minlast!":            0.01,
	"Lam_rpm_steg!":           10,
	"Lambdaint!":              0.001953125,
	"Lamd_tid!":               10,
	"Last_varv_st!":           10,
	"Limp_tryck_konst!":       0.01,
	"Lret_konst!":             0.00390625, // 1/256 //0.0078125, // 1/12,
	"Luft_kompfak!":           0.001953125,
	"Max_ratio_aut!":          0.01,
	"Max_regl_temp_1!":        0.01,
	"Max_regl_temp_2!":        0.01,
	"Max_rpm_gadapt!":         10,
	"Min_rpm_closed_loop!":    10,
	"Min_rpm_gadapt!":         10,
	"Misfire_map_x_axis!":     0.01,
	"Open_all_varv!":          10,
	"Open_loop_knock":         0.01,
	"Open_loop":               0.01,
	"Open_varv!":              10,
	"P_Manifold":              0.01,
	"P_Manifold10":            0.001,
	"PMCal_RpmIdleNomRefLim!": 10,
	"Press_rpm_lim!":          10,
	"Pressure map (AUT) scaled for 3 bar mapsensor": 0.012,
	"Pressure map scaled for 3 bar mapsensor":       0.012,
	"Purge_map_xaxis!":                              0.01,
	"Pwm_ind_rpm!":                                  10,
	"Reg_kon_mat":                                   0.1,
	"Reg_last!":                                     0.01,
	"Reg_varv!":                                     10,
	"Regl_tryck":                                    0.01,
	"Ret_delta_rpm!":                                10,
	"Ret_down_rpm!":                                 10,
	"Ret_fuel_fak!":                                 0.0009765625, // 128/25,
	"Ret_fuel_tab!":                                 0.0009765625, // 128/25,
	"Ret_up_rpm!":                                   10,
	"Retard_konst!":                                 0.00390625, //returnvalue = 0.0078125, // 1/12,
	"Rpm_dif!":                                      10,
	"Rpm_max!":                                      10,
	"Rpm_perf_max!":                                 10,
	"Rpm_perf_min!":                                 10,
	"Shift_load!":                                   0.01,
	"Shift_up_load_hyst!":                           0.01,
	"Sond_heat_tab":                                 0.01,
	"Start_detekt_rpm!":                             10,
	"Start_insp!":                                   0.004,     // 1/ ((256*256) / 260,
	"Start_proc!":                                   0.0078125, // 1/12,
	"Startvev_fak!":                                 0.125,     // 1/,
	"Tryck_mat_a!":                                  0.01,
	"Tryck_mat!":                                    0.01,
	"Tryck_vakt_tab!":                               0.01,
	"Turbo_knock_press":                             0.01, // ba,
	"Turbo_knock_tab":                               0.01,

	"BstKnkCal.fi_offsetXSP": 0.1,
}

func GetCorrectionfactor(name string) float64 {
	if val, exists := correctionFactors[name]; exists {
		return val
	}
	return 1
}

func GetPrecision(corrFac float64) int {
	precission := 0
	switch corrFac {
	case 0.1:
		precission = 1
	case 0.01, 0.00390625, 0.004:
		precission = 2
	case 0.001:
		precission = 3
	case 0.0009765625: // 1/1024
		precission = 4
	case 0.0078125: // 1/128
		precission = 3
	}
	return precission
}

/* t8 values
   if (symbolname == "KnkSoundRedCal.fi_OffsMa") returnvalue = 0.1,
   else if (symbolname == "IgnE85Cal.fi_AbsMap") returnvalue = 0.1,
   else if (symbolname == "MAFCal.cd_ThrottleMap") returnvalue = 0.0009765625,
   else if (symbolname == "TrqMastCal.Trq_NominalMap") returnvalue = 0.1,
   else if (symbolname == "TrqMastCal.Trq_MBTMAP") returnvalue = 0.1,
   else if (symbolname == "AfterStCal.StartMAP") returnvalue = 0.0009765625; // 1/1024
   else if (symbolname == "KnkFuelCal.EnrichmentMap") returnvalue = 0.0009765625; // 1/1024
   else if (symbolname == "AfterStCal.HotSoakMAP") returnvalue = 0.0009765625; // 1/1024
   else if (symbolname == "MAFCal.NormAdjustFacMap") returnvalue = 0.0078125; // 1/128
   else if (symbolname == "BFuelCal.LambdaOneFacMap") returnvalue = 0.0078125; // 1/128
   else if (symbolname == "BFuelCal.TempEnrichFacMap") returnvalue = 0.0078125; // 1/128
   else if (symbolname == "BFuelCal.E85TempEnrichFacMap") returnvalue = 0.0078125; // 1/128
   else if (symbolname == "AfterStCal.AmbientMAP") returnvalue = 0.0078125; // 1/128
   else if (symbolname == "FFFuelCal.KnkEnrichmentMAP") returnvalue = 0.0078125; // 1/128
   else if (symbolname == "FFFuelCal.TempEnrichFacMAP") returnvalue = 0.0078125; // 1/128
*/
