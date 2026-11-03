package com.v2ray.ang.fmt

import com.v2ray.ang.core.TorCore
import com.v2ray.ang.core.TorCoreManager
import com.v2ray.ang.dto.entities.ProfileItem
import com.v2ray.ang.enums.EConfigType
import com.v2ray.ang.util.Utils
import java.net.URI

object TorFmt : FmtBase() {

    enum class Problem {
        LISTEN_PORT_TAKEN,
        BRIDGES_MISSING,
    }

    fun parse(str: String): ProfileItem? {
        val config = ProfileItem.create(EConfigType.TOR)
        val uri = URI(Utils.fixIllegalUrl(str))
        val queryParam = if (uri.rawQuery.isNullOrEmpty()) emptyMap() else getQueryParam(uri)

        config.remarks = Utils.decodeURIComponent(uri.fragment.orEmpty()).ifEmpty { "Tor" }
        config.torSnowflake = parseBoolParam(queryParam["snowflake"], default = true)
        config.torLyrebird = parseBoolParam(queryParam["lyrebird"], default = true)
        config.torConjure = parseBoolParam(queryParam["conjure"], default = true)
        config.torAutoFetch = parseBoolParam(queryParam["auto_fetch"], default = true)
        config.torCustomBridges = queryParam["bridges"]
            ?.split(';')
            ?.map { it.trim() }
            ?.filter { it.isNotEmpty() }
            ?.joinToString("\n")
            ?.takeIf { it.isNotEmpty() }

        return config
    }

    fun toUri(config: ProfileItem): String {
        val core = TorCore.of(config)
        val query = linkedMapOf(
            "snowflake" to if (core.snowflake) "1" else "0",
            "lyrebird" to if (core.lyrebird) "1" else "0",
            "conjure" to if (core.conjure) "1" else "0",
            "auto_fetch" to if (core.autoFetch) "1" else "0",
        )
        core.customBridges.takeIf { it.isNotEmpty() }?.let { bridges ->
            query["bridges"] = bridges.joinToString(";")
        }

        val queryText = query.entries.joinToString("&") { "${it.key}=${Utils.encodeURIComponent(it.value)}" }
        return "?$queryText#${Utils.encodeURIComponent(config.remarks)}"
    }

    fun normalize(
        config: ProfileItem,
        takenPorts: Set<Int> = emptySet(),
        listenPort: Int = TorCoreManager.socksPort,
    ): Problem? {
        val bridges = TorCore.bridgeLines(config.torCustomBridges)
        config.torCustomBridges = bridges.joinToString("\n").takeIf { it.isNotEmpty() }

        val autoFetch = config.torAutoFetch != false
        val anyTransport = (config.torSnowflake != false) || (config.torLyrebird != false) || (config.torConjure != false)
        if (!autoFetch && bridges.isEmpty() && !anyTransport) {
            return Problem.BRIDGES_MISSING
        }
        if (listenPort in takenPorts) {
            return Problem.LISTEN_PORT_TAKEN
        }
        return null
    }

    private fun parseBoolParam(raw: String?, default: Boolean): Boolean = when (raw?.trim()?.lowercase()) {
        "1", "true", "yes", "on" -> true
        "0", "false", "no", "off" -> false
        else -> default
    }
}
