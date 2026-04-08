use schemars::JsonSchema;
use serde::Deserialize;
use zed::settings::ContextServerSettings;
use zed_extension_api::{
    self as zed, serde_json, Command, ContextServerConfiguration, ContextServerId, Project, Result,
};

struct MairuExtension;

#[derive(Debug, Deserialize, JsonSchema)]
struct MairuContextServerSettings {
    #[serde(default)]
    path: Option<String>,
}

impl zed::Extension for MairuExtension {
    fn new() -> Self {
        Self
    }

    fn context_server_command(
        &mut self,
        _context_server_id: &ContextServerId,
        project: &Project,
    ) -> Result<Command> {
        let settings = ContextServerSettings::for_project("mairu", project)?;
        
        let mut command = "mairu".to_string();
        
        if let Some(settings) = settings.settings {
            if let Ok(settings) = serde_json::from_value::<MairuContextServerSettings>(settings) {
                if let Some(path) = settings.path {
                    if !path.is_empty() {
                        command = path;
                    }
                }
            }
        }

        Ok(Command {
            command,
            args: vec!["mcp".into()],
            env: vec![],
        })
    }

    fn context_server_configuration(
        &mut self,
        _context_server_id: &ContextServerId,
        _project: &Project,
    ) -> Result<Option<ContextServerConfiguration>> {
        let installation_instructions = include_str!("../configuration/installation_instructions.md").to_string();
        
        let default_settings = include_str!("../configuration/default_settings.jsonc").to_string();
        let settings_schema =
            serde_json::to_string(&schemars::schema_for!(MairuContextServerSettings))
                .map_err(|e| e.to_string())?;

        Ok(Some(ContextServerConfiguration {
            installation_instructions,
            default_settings,
            settings_schema,
        }))
    }
}

zed::register_extension!(MairuExtension);
