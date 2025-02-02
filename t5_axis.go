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
		XDescription: "Relative throttle value",
		YDescription: "Rpm axis for several tables",
		ZDescription: "MAP",
		XFrom:        "Medeltrot",
		YFrom:        "Rpm",
	},
}
