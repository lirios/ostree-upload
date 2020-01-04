/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

use crate::receiver::Receiver;
use std;
use std::collections::HashMap;
use std::io;
use std::path::Path;
use std::sync::Arc;

// Config

#[derive(Deserialize, Debug, Clone)]
#[serde(rename_all = "kebab-case", deny_unknown_fields)]
pub struct Config {
    #[serde(default = "default_host")]
    pub host: String,
    #[serde(default = "default_port")]
    pub port: i32,
    #[serde(default = "default_repo_path")]
    pub repo_path: String,
}

fn default_host() -> String {
    String::from("127.0.0.1")
}

fn default_port() -> i32 {
    8080
}

fn default_repo_path() -> String {
    String::from("repo")
}

// AppState

#[derive(Clone)]
pub struct AppState {
    pub config: Arc<Config>,
    pub receiver: Arc<Receiver>,
    pub update_refs: HashMap<String, (String, String)>,
    pub received_objects: Vec<String>,
}

// Methods

pub fn load_config<P: AsRef<Path>>(path: P) -> io::Result<Config> {
    let config_contents = std::fs::read_to_string(path)?;
    let config_data: Config = serde_json::from_str(&config_contents)
        .map_err(|err| io::Error::new(io::ErrorKind::Other, err))?;

    Ok(config_data)
}
