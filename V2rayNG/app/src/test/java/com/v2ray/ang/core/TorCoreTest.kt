package com.v2ray.ang.core

import com.v2ray.ang.AppConfig
import com.v2ray.ang.dto.entities.ProfileItem
import com.v2ray.ang.enums.EConfigType
import com.v2ray.ang.fmt.TorFmt
import com.v2ray.ang.ui.server.ServerUiState
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertNull
import org.junit.jupiter.api.Test

class TorCoreTest {

    @Test
    fun buildsArgumentsAndActiveTransportsFromProfile() {
        val profile = ProfileItem.create(EConfigType.TOR).apply {
            remarks = "Tor Bridge Node"
            torSnowflake = true
            torLyrebird = false
            torConjure = true
            torAutoFetch = false
            torCustomBridges = """
                # comment
                Bridge conjure 23.128.36.60:80 A13CE1DE88F96022C0CE6CCF6436FC6F5401D35B url=https://example.org
                snowflake 192.0.2.3:80 2B280B23E1107BB62ABFC40DDCC8824814F80A72
            """.trimIndent()
        }

        val core = TorCore.of(profile, listenPort = 19050, defaults = TorSettings()).through(19053)
        assertEquals(listOf("Snowflake", "Conjure"), core.activeTransports)
        assertEquals(
            listOf(
                "--bind", "127.0.0.1:19050",
                "--snowflake=true",
                "--lyrebird=false",
                "--conjure=true",
                "--auto-fetch=false",
                "--bridge", "conjure 23.128.36.60:80 A13CE1DE88F96022C0CE6CCF6436FC6F5401D35B url=https://example.org",
                "--bridge", "snowflake 192.0.2.3:80 2B280B23E1107BB62ABFC40DDCC8824814F80A72",
                "--upstream", "socks5://127.0.0.1:19053",
            ),
            core.arguments,
        )
    }

    @Test
    fun validatesListenPortAgainstLocalProxyAndAether() {
        assertEquals(AppConfig.PORT_TOR_SOCKS.toInt(), TorCoreManager.listenPortOf(null))
        assertEquals(AppConfig.PORT_TOR_SOCKS.toInt(), TorCoreManager.listenPortOf("80"))
        assertEquals(19050, TorCoreManager.listenPortOf("19050"))

        assertNull(TorCoreManager.listenPortProblem("19050", setOf(10808, 10809), aetherPort = 10819))
        assertEquals(
            TorCoreManager.ListenPortProblem.NOT_A_PORT,
            TorCoreManager.listenPortProblem("70000", setOf(10808)),
        )
        assertEquals(
            TorCoreManager.ListenPortProblem.LOCAL_PROXY,
            TorCoreManager.listenPortProblem("10808", setOf(10808, 10809), aetherPort = 10819),
        )
        assertEquals(
            TorCoreManager.ListenPortProblem.AETHER_PORT,
            TorCoreManager.listenPortProblem("10820", setOf(10808), aetherPort = 10819),
        )
    }

    @Test
    fun parsesAndSerializesTorUri() {
        val original = ProfileItem.create(EConfigType.TOR).apply {
            remarks = "My Tor"
            torSnowflake = false
            torLyrebird = true
            torConjure = true
            torAutoFetch = true
            torCustomBridges = "obfs4 192.0.2.1:443 AAAA cert=xyz iat-mode=0"
        }

        val uri = EConfigType.TOR.protocolScheme + TorFmt.toUri(original)
        val parsed = TorFmt.parse(uri)
        assertNotNull(parsed)
        assertEquals("My Tor", parsed!!.remarks)
        assertEquals(false, parsed.torSnowflake)
        assertEquals(true, parsed.torLyrebird)
        assertEquals(true, parsed.torConjure)
        assertEquals(true, parsed.torAutoFetch)
        assertEquals("obfs4 192.0.2.1:443 AAAA cert=xyz iat-mode=0", parsed.torCustomBridges)
    }

    @Test
    fun serverUiStatePreservesTorFields() {
        val initial = ProfileItem.create(EConfigType.TOR).apply {
            remarks = "Tor Node"
            torSnowflake = true
            torLyrebird = false
            torConjure = true
            torAutoFetch = false
            torCustomBridges = "conjure 23.128.36.60:80 AAAA url=https://example.org"
        }

        val state = ServerUiState.fromProfileItem(initial)
        val rebuilt = state.toProfileItem(initial)
        assertEquals(EConfigType.TOR, rebuilt.configType)
        assertEquals(true, rebuilt.torSnowflake)
        assertEquals(false, rebuilt.torLyrebird)
        assertEquals(true, rebuilt.torConjure)
        assertEquals(false, rebuilt.torAutoFetch)
        assertEquals("conjure 23.128.36.60:80 AAAA url=https://example.org", rebuilt.torCustomBridges)
    }
}
