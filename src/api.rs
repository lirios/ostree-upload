/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::PathBuf;

pub const REV_NULL: &str = "0000000000000000000000000000000000000000000000000000000000000000";

#[derive(Debug, Serialize, Deserialize)]
pub struct Info {
    pub mode: String,
    pub refs: HashMap<String, String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UpdateRequest {
    pub refs: HashMap<String, (String, String)>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct NeededObject {
    pub rev: String,
    pub object_name: String,
    pub object_path: PathBuf,
    pub checksum: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct MissingObjectsArgs {
    pub wanted: Vec<NeededObject>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct MissingObjectsResponse {
    pub missing: Vec<NeededObject>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Status {
    pub status: bool,
    pub message: Option<String>,
}
