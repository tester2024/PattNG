package com.v2ray.ang.core

import com.v2ray.ang.AppConfig
import com.v2ray.ang.dto.entities.ProfileItem

data class TorSettings(
    val listenPort: Int = AppConfig.PORT_TOR_SOCKS.toInt(),
    val snowflake: Boolean = true,
    val lyrebird: Boolean = true,
    val conjure: Boolean = true,
    val autoFetch: Boolean = true,
    val customBridges: String = "",
)

data class TorCore(
    val port: Int = AppConfig.PORT_TOR_SOCKS.toInt(),
    val snowflake: Boolean = true,
    val lyrebird: Boolean = true,
    val conjure: Boolean = true,
    val autoFetch: Boolean = true,
    val customBridges: List<String> = emptyList(),
    val upstreamPort: Int? = null,
) {

    val arguments: List<String>
        get() = buildList {
            addAll(listOf("--bind", "${AppConfig.LOOPBACK}:$port"))
            add("--snowflake=$snowflake")
            add("--lyrebird=$lyrebird")
            add("--conjure=$conjure")
            add("--auto-fetch=$autoFetch")
            customBridges.forEach { line ->
                addAll(listOf("--bridge", line))
            }
            upstreamPort?.let { upstream ->
                addAll(listOf("--upstream", "socks5://${AppConfig.LOOPBACK}:$upstream"))
            }
        }

    val command: String
        get() = (listOf(COMMAND_NAME) + arguments).joinToString(" ", transform = ::quoted)

    val activeTransports: List<String>
        get() = buildList {
            if (snowflake) add("Snowflake")
            if (lyrebird) add("Lyrebird")
            if (conjure) add("Conjure")
        }

    fun through(port: Int): TorCore = copy(upstreamPort = port)

    companion object {
        const val COMMAND_NAME = "tor-core"

        fun of(
            profile: ProfileItem,
            listenPort: Int = TorCoreManager.socksPort,
            defaults: TorSettings = TorCoreManager.settingsSource(),
        ): TorCore {
            val rawBridges = profile.torCustomBridges?.takeIf { it.isNotBlank() } ?: defaults.customBridges
            return TorCore(
                port = listenPort,
                snowflake = profile.torSnowflake ?: defaults.snowflake,
                lyrebird = profile.torLyrebird ?: defaults.lyrebird,
                conjure = profile.torConjure ?: defaults.conjure,
                autoFetch = profile.torAutoFetch ?: defaults.autoFetch,
                customBridges = bridgeLines(rawBridges),
            )
        }

        fun bridgeLines(text: String?): List<String> =
            text.orEmpty()
                .replace(';', '\n')
                .lineSequence()
                .map { it.trim() }
                .filter { it.isNotEmpty() && !it.startsWith('#') }
                .map { line ->
                    when {
                        line.startsWith("Bridge ", ignoreCase = true) -> line.substring(7).trim()
                        else -> line
                    }
                }
                .filter { it.isNotEmpty() && !it.startsWith('-') }
                .distinct()
                .toList()

        private fun quoted(word: String): String =
            if (word.isEmpty() || word.any(Char::isWhitespace)) "\"$word\"" else word
    }
}
