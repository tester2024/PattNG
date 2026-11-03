package com.v2ray.ang.ui.server

import android.app.Application
import androidx.lifecycle.viewModelScope
import com.v2ray.ang.AppConfig
import com.v2ray.ang.core.TorCoreManager
import com.v2ray.ang.handler.SettingsManager
import com.v2ray.ang.ui.base.BaseViewModel
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class ServerTorViewModel(application: Application) : BaseViewModel(application) {

    private val _isCoreAvailable = MutableStateFlow(true)
    val isCoreAvailable: StateFlow<Boolean> = _isCoreAvailable.asStateFlow()

    private val _isLyrebirdAvailable = MutableStateFlow(true)
    val isLyrebirdAvailable: StateFlow<Boolean> = _isLyrebirdAvailable.asStateFlow()

    private val _isConjureAvailable = MutableStateFlow(true)
    val isConjureAvailable: StateFlow<Boolean> = _isConjureAvailable.asStateFlow()

    private val _listenPort = MutableStateFlow(AppConfig.PORT_TOR_SOCKS.toInt())
    val listenPort: StateFlow<Int> = _listenPort.asStateFlow()

    private val _takenPorts = MutableStateFlow<Set<Int>>(emptySet())
    val takenPorts: StateFlow<Set<Int>> = _takenPorts.asStateFlow()

    init {
        refreshEnvironment()
    }

    fun refreshEnvironment() {
        viewModelScope.launch {
            val app = getApplication<Application>()
            val snapshot = withContext(Dispatchers.IO) {
                EnvironmentSnapshot(
                    coreAvailable = TorCoreManager.isSupported(app),
                    lyrebirdAvailable = TorCoreManager.isLyrebirdSupported(app),
                    conjureAvailable = TorCoreManager.isConjureSupported(app),
                    listenPort = SettingsManager.getTorListenPort(),
                    takenPorts = SettingsManager.getLocalProxyPorts(),
                )
            }
            _isCoreAvailable.value = snapshot.coreAvailable
            _isLyrebirdAvailable.value = snapshot.lyrebirdAvailable
            _isConjureAvailable.value = snapshot.conjureAvailable
            _listenPort.value = snapshot.listenPort
            _takenPorts.value = snapshot.takenPorts
        }
    }

    private data class EnvironmentSnapshot(
        val coreAvailable: Boolean,
        val lyrebirdAvailable: Boolean,
        val conjureAvailable: Boolean,
        val listenPort: Int,
        val takenPorts: Set<Int>,
    )
}
