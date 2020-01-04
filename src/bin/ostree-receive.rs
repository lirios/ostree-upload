/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

extern crate clap;
extern crate dotenv;
extern crate env_logger;
extern crate ostreereceiver;

use actix_web::{self, http, middleware, web, App, HttpServer};
use dotenv::dotenv;
use ostreereceiver::app::AppState;
use ostreereceiver::server;
use std::collections::HashMap;
use std::env;
use std::path::Path;
use std::path::PathBuf;
use std::sync::Arc;
use std::sync::Mutex;

#[actix_rt::main]
async fn main() -> std::io::Result<()> {
    if env::var("RUST_LOG").is_err() {
        env::set_var("RUST_LOG", "info");
    }
    env_logger::init();

    dotenv().ok();

    let mut config_path =
        PathBuf::from(env::var("OSTREE_RECEIVE_CONFIG").unwrap_or("config.json".to_string()));

    let matches = clap::App::new("ostree-receive")
        .version(env!("CARGO_PKG_VERSION"))
        .about("Receive OStree objects")
        .author("Pier Luigi Fiorini")
        .arg(
            clap::Arg::with_name("config")
                .short("c")
                .long("config")
                .takes_value(true)
                .help("Path to the configuration file"),
        )
        .get_matches();
    if matches.is_present("config") {
        config_path = PathBuf::from(matches.value_of("config").unwrap());
    }

    let config = ostreereceiver::load_config(&config_path);

    let state = Arc::new(Mutex::new(AppState {
        config: config.clone(),
        receiver: ostreereceiver::create_receiver(Path::new(&config.repo_path)),
        update_refs: HashMap::new(),
        received_objects: Vec::new(),
    }));

    let http_server = HttpServer::new(move || {
        App::new()
            .wrap(middleware::Logger::default())
            .wrap(middleware::Compress::new(
                http::header::ContentEncoding::Identity,
            ))
            .data(state.clone())
            .service(
                web::scope("/api/v1")
                    .service(web::resource("/ping").route(web::get().to(server::ping)))
                    .service(web::resource("/info").route(web::get().to(server::get_info)))
                    .service(web::resource("/update").route(web::post().to(server::update)))
                    .service(
                        web::resource("/missing_objects")
                            .data(web::JsonConfig::default().limit(1024 * 1024 * 10))
                            .route(web::get().to(server::objects)),
                    )
                    .service(web::resource("/upload").route(web::post().to(server::upload)))
                    .service(web::resource("/done").route(web::post().to(server::done))),
            )
    });

    let bind_to = format!("{}:{}", config.host, config.port);
    http_server.keep_alive(75).bind(&bind_to)?.run().await
}
