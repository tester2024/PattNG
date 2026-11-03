package com.v2ray.ang.dto

import com.v2ray.ang.core.AetherCore
import com.v2ray.ang.core.TorCore

data class ConfigResult(
    var status: Boolean,
    var guid: String? = null,
    var content: String = "",
    var errorMessage: String = "",
    /** True when [errorMessage] is a localized resource string meant for the screen. */
    var localizedError: Boolean = false,
    /** The Aether core the configuration runs on, when it has an Aether outbound; the daemon starts it. */
    var aetherCore: AetherCore? = null,
    /** The standalone Tor core the configuration runs on, when it has a Tor outbound; the daemon starts it. */
    var torCore: TorCore? = null,
)
