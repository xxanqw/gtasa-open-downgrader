package mods

// Pinned archive members: no third-party artwork/binaries are embedded.
var interfacePacks = map[string]interfaceSpec{
	OriginalHQIcons: {folder: "modloader/OriginalHQRadarIcons", checksum: "c50caa241cf0773729f790ef72c455589c38f93eff9ae84bbe132fd8646f9623", members: map[string]string{
		"Readme (or die).txt":                "README-upstream.txt",
		"SA Original Radar Icons HQ/hud.txd": "models/hud.txd",
	}, hashes: map[string]string{
		"README-upstream.txt": "fa091a74d9b66ab34492db3312ce4d5c67c091130d5ea14fea26f452e33152eb",
		"models/hud.txd":      "38ee3e20a70dc975879c66d63e68bee03e5f7c457d2f79fb006d17bd1b0595e6",
	}},
	AtmosphereHUD: {folder: "modloader/AtmosphereHUD", checksum: "f946f493b1dd35222cdcebe2de8a6366c4857de7d4d11658e636f6f91f8ebcc1", members: map[string]string{
		"Atmosphere Interface Pack/models/LD_RCE1.txd":     "models/LD_RCE1.txd",
		"Atmosphere Interface Pack/models/LD_RCE2.txd":     "models/LD_RCE2.txd",
		"Atmosphere Interface Pack/models/LD_RCE3.txd":     "models/LD_RCE3.txd",
		"Atmosphere Interface Pack/models/LD_RCE4.txd":     "models/LD_RCE4.txd",
		"Atmosphere Interface Pack/models/LD_RCE5.txd":     "models/LD_RCE5.txd",
		"Atmosphere Interface Pack/models/fronten_pc.txd":  "models/fronten_pc.txd",
		"Atmosphere Interface Pack/models/pcbtns.txd":      "models/pcbtns.txd",
		"Atmosphere Interface Pack/models/txd/LD_BEAT.txd": "models/txd/LD_BEAT.txd",
		"Atmosphere Interface Pack/models/txd/LD_CARD.txd": "models/txd/LD_CARD.txd",
		"Atmosphere Interface Pack/models/txd/LD_CHAT.txd": "models/txd/LD_CHAT.txd",
		"Atmosphere Interface Pack/models/txd/LD_DRV.txd":  "models/txd/LD_DRV.txd",
		"Atmosphere Interface Pack/models/txd/LD_POKE.txd": "models/txd/LD_POKE.txd",
		"Atmosphere Interface Pack/models/txd/LD_POOL.txd": "models/txd/LD_POOL.txd",
		"Atmosphere Interface Pack/models/txd/LD_ROUL.txd": "models/txd/LD_ROUL.txd",
		"Atmosphere Interface Pack/models/txd/LD_SLOT.txd": "models/txd/LD_SLOT.txd",
		"Readme (or die).txt":                              "README-upstream.txt",
	}, hashes: map[string]string{
		"models/LD_RCE1.txd":     "9e9a38f68905cfa3f70e54d38d563349aa45691ebf1c07ff55af242557a41d0c",
		"models/LD_RCE2.txd":     "b75a1136d63297099c43e004851a1e2f08ad6e456aab77f5e52fa715be8ee1f6",
		"models/LD_RCE3.txd":     "bfd8a54795baf879aa0472462da40fe89af972bd579fb27a1c7915fd1f79a537",
		"models/LD_RCE4.txd":     "0a4853e5cfe61a2807e9207435decdf79f8e279e97fb72e665fd9dc0f2ea7f79",
		"models/LD_RCE5.txd":     "af956c6bf4923d309a4cf8e3a3cb1e9ec4735f5e5747c0d410ce8178c92f02ce",
		"models/fronten_pc.txd":  "a026c485e4ea0a04367de00857792640a45aac0bce5c6007c8777021f3b8fc86",
		"models/pcbtns.txd":      "bac1e81ffb33e77609080db00fa3b370ea4e867dc9e935846fe969a40b5a05cd",
		"models/txd/LD_BEAT.txd": "9d3a79794505731f38117d22d2ae6462a01a3edc39bf93bc47eda51250606ef5",
		"models/txd/LD_CARD.txd": "f10e499702dd1d99a0d1dd0b8d89df883d39e4e4591399727c9edc8b59f9a236",
		"models/txd/LD_CHAT.txd": "b1b5f1ef5a54ba5b2fe676e0de6129114170405e51dbbf4c042405ae82ada9fe",
		"models/txd/LD_DRV.txd":  "1df4a4c092aaa55be8262e4014a2d12d78e891acb458999fbf3cbc555ab6f34d",
		"models/txd/LD_POKE.txd": "8915e56deb1161df2dc5ebb688cecababafc723b0a8374456c950a9be6ec1865",
		"models/txd/LD_POOL.txd": "5c02518733f3a09be4be3de2333be8a22ca9c1b34d70cf70efe4bed3af47651a",
		"models/txd/LD_ROUL.txd": "39b0ea24ce580ad1be54c396d1f8a8489f848e93d20fe2adc5d79f3fb3a2d344",
		"models/txd/LD_SLOT.txd": "e5ae3a48cc1e2fdad20c55ed3e039b695e9f96abae10024061555ca329f17203",
		"README-upstream.txt":    "c764aa4a79e92355249dd544b982a44302a6c98c6ab1f974b143b0a3ed044adf",
	}},
	NadalaoWeapons: {folder: "modloader/NadalaoWeaponIcons", checksum: "20d3fd1282a99f6d7a785961cdc228c088cee28f15cf1364ed8ca5b5aa4de14a", members: map[string]string{
		"Readme (or die).txt":                    "README-icons.txt",
		"Weapon Icons TXD/WeaponIcons.txd":       "WeaponIcons.txd",
		"Weapon Icons TXD/WeaponIconsTXD.SA.asi": "WeaponIconsTXD.SA.asi",
		"Readme-helper.txt":                      "README-helper.txt",
	}, hashes: map[string]string{
		"README-icons.txt":      "80f4fcee2fa5f8f3a73cfbb7fcc886eb0bcb2d24e96e54a8fcd91f4874cf25c0",
		"WeaponIcons.txd":       "25aa494b657eecf10d5bf7f2681c957f6fd747542aa529dfc808aa4810a5da0b",
		"WeaponIconsTXD.SA.asi": "4f8dc3dea5dc5cdd1a241c00e823ba3f1be3ad480b83fba296a30d3f941ea8de",
		"README-helper.txt":     "4317da6e7027359cf2a7ce2ae42f4a1700a57c4584c28246184e4f1ac3428765",
	}},
	HDRadarMap: radarRasterSpec(),
}
