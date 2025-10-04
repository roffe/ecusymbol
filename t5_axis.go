package symbol

var axisT5 = AxisInformation{
	"Insp_mat!": Axis{
		X:            "Fuel_map_xaxis!",
		Y:            "Fuel_map_yaxis!",
		Z:            "Insp_mat!",
		XDescription: "MAP",
		YDescription: "RPM",
		ZDescription: "VE table",
		XFrom:        "P_medel",
		YFrom:        "Rpm",
	},
	"Batt_korr_tab!": Axis{
		X:            "",
		Y:            "",
		Z:            "Batt_korr_tab!",
		XDescription: "",
		YDescription: "",
		ZDescription: "Injector dead time table",
		XFrom:        "",
		YFrom:        "Batt_volt",
	},
	"Tryck_mat!": Axis{
		X:            "Pwm_ind_trot!",
		Y:            "Pwm_ind_rpm!",
		Z:            "Tryck_mat!",
		XDescription: "Throttle",
		YDescription: "Rpm",
		ZDescription: "Boost",
		XFrom:        "Medeltrot",
		YFrom:        "Rpm",
	},
	"Ign_map_0!": Axis{
		X:            "Ign_map_0_x_axis!",
		Y:            "Ign_map_0_y_axis!",
		Z:            "Ign_map_0!",
		XDescription: "MAP",
		YDescription: "RPM",
		ZDescription: "Ign angel",
		XFrom:        "P_medel",
		YFrom:        "Rpm",
	},
	"P_fors!": Axis{
		X:            "Reg_last!",
		Y:            "Reg_varv!",
		Z:            "P_fors!",
		XDescription: "Pressure Error",
		YDescription: "Rpm",
		ZDescription: "P-Gain",
		XFrom:        "P_medel",
		YFrom:        "Rpm",
	},
	"I_fors!": Axis{
		X:            "Reg_last!",
		Y:            "Reg_varv!",
		Z:            "I_fors!",
		XDescription: "Pressure Error",
		YDescription: "Rpm",
		ZDescription: "I-Gain",
		XFrom:        "P_medel",
		YFrom:        "Rpm",
	},
	"D_fors!": Axis{
		X:            "Reg_last!",
		Y:            "Reg_varv!",
		Z:            "D_fors!",
		XDescription: "Pressure Error",
		YDescription: "Rpm",
		ZDescription: "D-Gain",
		XFrom:        "P_medel",
		YFrom:        "Rpm",
	},
	"Reg_kon_mat!": Axis{
		X:            "Pwm_ind_trot!",
		Y:            "Pwm_ind_rpm!",
		Z:            "Reg_kon_mat!",
		XDescription: "Throttle",
		YDescription: "Rpm",
		ZDescription: "BCV Constant",
		XFrom:        "Medeltrot",
		YFrom:        "Rpm",
	},
}

/*
Symbol	Description	X-axis	X-axis description	Y-axis	Y-axis description
Reg_kon_mat!	Boost Regulation Map, BCV Constant (manual).	Pwm_ind_trot!	Relative throttle value	Pwm_ind_rpm!	Rpm axis for several tables
*/
