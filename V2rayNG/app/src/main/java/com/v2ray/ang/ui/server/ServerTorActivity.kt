package com.v2ray.ang.ui.server

import android.os.Bundle
import androidx.activity.viewModels
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import com.v2ray.ang.R
import com.v2ray.ang.core.TorCore
import com.v2ray.ang.dto.entities.ProfileItem
import com.v2ray.ang.enums.EConfigType
import com.v2ray.ang.extension.toast
import com.v2ray.ang.fmt.TorFmt
import com.v2ray.ang.ui.compose.FormTextField
import com.v2ray.ang.ui.compose.SettingsSwitchItem
import kotlinx.coroutines.launch

class ServerTorActivity : BaseServerActivity() {

    override val serverConfigType: EConfigType = EConfigType.TOR

    private val viewModel: ServerTorViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.RESUMED) {
                viewModel.refreshEnvironment()
            }
        }
    }

    @Composable
    override fun EditorContent() {
        val uiState = rememberSaveable(saver = ServerUiState.Saver) {
            ServerUiState.from(initialConfig = initialConfig)
        }.apply {
            configType = serverConfigType
        }
        val isCoreAvailable by viewModel.isCoreAvailable.collectAsStateWithLifecycle()
        val listenPort by viewModel.listenPort.collectAsStateWithLifecycle()

        ServerEditorScaffold(
            title = serverConfigType.toString(),
            onSaveClick = { saveServer(uiState) },
        ) {
            FormTextField(
                label = stringResource(R.string.server_lab_remarks),
                value = uiState.remarks,
                onValueChange = { uiState.remarks = it },
                isError = uiState.isRemarksError,
            )
            SettingsSwitchItem(
                title = stringResource(R.string.tor_lab_snowflake),
                summary = stringResource(R.string.tor_summary_snowflake),
                checked = uiState.torSnowflake,
                onCheckedChange = { uiState.torSnowflake = it },
            )
            SettingsSwitchItem(
                title = stringResource(R.string.tor_lab_lyrebird),
                summary = stringResource(R.string.tor_summary_lyrebird),
                checked = uiState.torLyrebird,
                onCheckedChange = { uiState.torLyrebird = it },
            )
            SettingsSwitchItem(
                title = stringResource(R.string.tor_lab_conjure),
                summary = stringResource(R.string.tor_summary_conjure),
                checked = uiState.torConjure,
                onCheckedChange = { uiState.torConjure = it },
            )
            SettingsSwitchItem(
                title = stringResource(R.string.tor_lab_auto_fetch),
                summary = stringResource(R.string.tor_summary_auto_fetch),
                checked = uiState.torAutoFetch,
                onCheckedChange = { uiState.torAutoFetch = it },
            )
            FormTextField(
                label = stringResource(R.string.tor_lab_custom_bridges),
                value = uiState.torCustomBridges,
                onValueChange = { uiState.torCustomBridges = it },
                placeholder = stringResource(R.string.aether_hint_tor_bridge_lines),
                maxLines = 6,
            )
            if (!isCoreAvailable) {
                Text(
                    text = stringResource(R.string.tor_unsupported_abi),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(horizontal = 16.dp),
                )
            }
            val builtCommand = TorCore.of(uiState.toProfileItem(initialConfig), listenPort).command
            FormTextField(
                label = stringResource(R.string.aether_lab_command),
                value = builtCommand,
                onValueChange = {},
                enabled = false,
                maxLines = 6,
            )
        }
    }

    override fun validateBasicConfig(state: ServerUiState): Boolean {
        val remarksErr = state.remarks.isBlank()
        state.isRemarksError = remarksErr
        if (remarksErr) {
            toast(R.string.server_lab_remarks)
            return false
        }
        return true
    }

    override fun validateProtocolConfig(config: ProfileItem): Boolean {
        val takenPorts = viewModel.takenPorts.value
        val listenPort = viewModel.listenPort.value
        return when (TorFmt.normalize(config, takenPorts, listenPort)) {
            null -> true
            TorFmt.Problem.LISTEN_PORT_TAKEN -> {
                toast(getString(R.string.aether_listen_port_taken, listenPort))
                false
            }
            TorFmt.Problem.BRIDGES_MISSING -> {
                toast(R.string.aether_tor_bridges_missing)
                false
            }
        }
    }
}
