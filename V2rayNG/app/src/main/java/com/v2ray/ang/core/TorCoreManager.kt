package com.v2ray.ang.core

import android.content.Context
import com.v2ray.ang.AppConfig
import com.v2ray.ang.handler.MmkvManager
import com.v2ray.ang.util.LogUtil
import java.io.File
import java.io.IOException
import java.util.concurrent.Executors
import kotlin.concurrent.thread

object TorCoreManager {

    private const val BINARY_NAME = "libtor.so"
    private const val LYREBIRD_BINARY_NAME = "liblyrebird.so"
    private const val CONJURE_BINARY_NAME = "libconjure.so"
    private const val ARTI_BINARY_NAME = "libaether.so"
    private const val WORK_DIR_NAME = "tor-core"

    private const val FIRST_LISTEN_PORT = 1024
    private const val LAST_LISTEN_PORT = 65535
    private const val PROBE_TIMEOUT_MS = 1000
    private const val READY_POLL_MS = 500L

    internal const val LYREBIRD_BIN_ENV = "TOR_CORE_LYREBIRD_BIN"
    internal const val CONJURE_BIN_ENV = "TOR_CORE_CONJURE_BIN"
    internal const val ARTI_BIN_ENV = "TOR_CORE_ARTI_BIN"

    private val socksAnswers = SocksAnswers(PROBE_TIMEOUT_MS)

    private val lifecycle = Executors.newSingleThreadExecutor { task ->
        Thread(task, "tor-core").apply { isDaemon = true }
    }

    @Volatile
    var listenPortSource: () -> Int = { AppConfig.PORT_TOR_SOCKS.toInt() }

    @Volatile
    var settingsSource: () -> TorSettings = { TorSettings() }

    val socksPort: Int get() = listenPortSource()

    @Volatile
    private var session: Session? = null

    val isRunning: Boolean get() = session != null

    fun isSupported(context: Context): Boolean = binary(context).canExecute()

    fun isLyrebirdSupported(context: Context): Boolean = lyrebirdBinary(context).canExecute()

    fun isConjureSupported(context: Context): Boolean = conjureBinary(context).canExecute()

    internal fun listenPortOf(text: String?): Int =
        text?.trim()?.toIntOrNull()?.takeIf { it in FIRST_LISTEN_PORT..LAST_LISTEN_PORT }
            ?: AppConfig.PORT_TOR_SOCKS.toInt()

    internal fun listenPortProblem(
        text: String,
        localPorts: Set<Int>,
        aetherPort: Int = AetherCoreManager.socksPort,
    ): ListenPortProblem? {
        val port = text.trim().toIntOrNull()?.takeIf { it in FIRST_LISTEN_PORT..LAST_LISTEN_PORT }
            ?: return ListenPortProblem.NOT_A_PORT
        if (port in localPorts) return ListenPortProblem.LOCAL_PROXY
        if (port in aetherPort..(aetherPort + 3)) return ListenPortProblem.AETHER_PORT
        return null
    }

    internal enum class ListenPortProblem {
        NOT_A_PORT,
        LOCAL_PROXY,
        AETHER_PORT,
    }

    @Synchronized
    fun start(context: Context, core: TorCore, onExit: () -> Unit) {
        stop()
        val appContext = context.applicationContext
        val logLevel = AetherCoreManager.coreLogLevel(MmkvManager.decodeSettingsString(AppConfig.PREF_LOGLEVEL))
        val arguments = core.arguments + listOf("--log-level", logLevel)
        val next = Session(core.port, arguments, appContext, onExit)
        session = next
        lifecycle.execute { open(next, appContext, arguments) }
    }

    @Synchronized
    fun stop() {
        val current = session ?: return
        session = null
        lifecycle.execute {
            current.process?.let { AetherCoreManager.endProcess(it) }
            AetherCoreManager.reapOrphans(current.context)
        }
    }

    suspend fun awaitListening(timeoutMs: Long): Boolean =
        AetherCoreManager.awaitReady(
            timeoutMs = timeoutMs,
            pollMs = READY_POLL_MS,
            running = { isRunning },
            listening = { session?.let { socksAnswers.answers(it.port) } == true },
        )

    internal fun startProcess(context: Context, arguments: List<String>): Process {
        val workDir = File(context.filesDir, WORK_DIR_NAME).apply { mkdirs() }
        val fullArgs = arguments + listOf("--data-dir", workDir.absolutePath)
        val builder = ProcessBuilder(listOf(binary(context).absolutePath) + fullArgs)
            .directory(workDir)
            .redirectErrorStream(true)

        builder.environment().apply {
            put(AetherCoreManager.OWNER_ENV, android.os.Process.myPid().toString())
            lyrebirdBinary(context).takeIf { it.canExecute() }?.let { put(LYREBIRD_BIN_ENV, it.absolutePath) }
            conjureBinary(context).takeIf { it.canExecute() }?.let { put(CONJURE_BIN_ENV, it.absolutePath) }
            artiBinary(context).takeIf { it.canExecute() }?.let { put(ARTI_BIN_ENV, it.absolutePath) }
            AetherCoreManager.certificateDirectories(File::isDirectory)?.let {
                put(AetherCoreManager.CERT_DIR_ENV, it)
            }
            put("HOME", workDir.absolutePath)
            put("TMPDIR", context.cacheDir.absolutePath)
        }

        return builder.start()
    }

    private fun open(target: Session, context: Context, arguments: List<String>) {
        if (session !== target) return
        val process = try {
            startProcess(context, arguments)
        } catch (e: IOException) {
            LogUtil.e(AppConfig.TAG, "TorCore: failed to launch tor-core", e)
            if (release(target)) target.onExit()
            return
        }
        target.process = process
        thread(name = "tor-core-output", isDaemon = true) { watch(target, process) }
    }

    private fun watch(target: Session, process: Process) {
        try {
            process.inputStream.bufferedReader().forEachLine { line ->
                AetherCoreManager.relay(line, "tor-core")
            }
        } catch (e: IOException) {
            LogUtil.d(AppConfig.TAG, "TorCore: output closed: ${e.message}")
        }
        val exitCode = process.waitFor()
        if (!release(target)) return
        AetherCoreManager.reapOrphans(target.context)
        LogUtil.e(AppConfig.TAG, "TorCore: the core exited on its own with code $exitCode")
        target.onExit()
    }

    @Synchronized
    private fun release(target: Session): Boolean {
        if (session !== target) return false
        session = null
        return true
    }

    private fun binary(context: Context): File =
        File(context.applicationInfo.nativeLibraryDir, BINARY_NAME)

    private fun lyrebirdBinary(context: Context): File =
        File(context.applicationInfo.nativeLibraryDir, LYREBIRD_BINARY_NAME)

    private fun conjureBinary(context: Context): File =
        File(context.applicationInfo.nativeLibraryDir, CONJURE_BINARY_NAME)

    private fun artiBinary(context: Context): File =
        File(context.applicationInfo.nativeLibraryDir, ARTI_BINARY_NAME)

    private class Session(
        val port: Int,
        val arguments: List<String>,
        val context: Context,
        val onExit: () -> Unit,
    ) {
        var process: Process? = null
    }
}
