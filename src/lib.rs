/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

extern crate actix;
extern crate actix_multipart;
extern crate actix_web;
extern crate clap;
extern crate env_logger;
extern crate futures;
extern crate indicatif;
#[macro_use]
extern crate failure;
extern crate serde;
#[macro_use]
extern crate serde_json;
#[macro_use]
extern crate serde_derive;
extern crate jsonwebtoken as jwt;
#[macro_use]
extern crate log;
extern crate reqwest;
extern crate sha2;

pub mod api;
pub mod app;
pub mod client;
pub mod errors;
pub mod pusher;
pub mod receiver;
pub mod server;

use app::Config;
use std::path::Path;
use std::sync::Arc;

pub fn load_config(path: &Path) -> Arc<Config> {
    let config_data =
        app::load_config(&path).expect(&format!("Failed to read config file {:?}", &path));
    Arc::new(config_data)
}

pub fn create_receiver(repo_path: &Path) -> Arc<receiver::Receiver> {
    let receiver = receiver::Receiver::new(&repo_path)
        .expect(&format!("Failed to open the repository {:?}", &repo_path));
    Arc::new(receiver)
}
