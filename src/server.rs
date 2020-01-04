/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

use crate::api;
use crate::app::AppState;
use crate::errors::ApiError;
use actix_multipart::Multipart;
use actix_web::{web, HttpResponse, Result};
use futures::StreamExt;
use log::debug;
use sha2::{Digest, Sha256};
use std::error::Error;
use std::fs;
use std::io::{self, Write};
use std::path::Path;
use std::sync::Arc;
use std::sync::Mutex;

fn calculate_checksum(path: &Path) -> io::Result<String> {
    let mut file = fs::File::open(&path)?;
    let mut hasher = Sha256::new();
    io::copy(&mut file, &mut hasher)?;
    let hash = hasher.result();
    let hex = hash.as_ref().iter().map(|b| format!("{:x}", b)).collect();
    Ok(hex)
}

pub async fn ping() -> Result<web::HttpResponse, ApiError> {
    Ok(HttpResponse::Ok().body("{}".to_string()))
}

pub async fn get_info(
    state: web::Data<Arc<Mutex<AppState>>>,
) -> Result<web::Json<api::Info>, ApiError> {
    let state = state.lock().unwrap();
    let result = state
        .receiver
        .get_info()
        .map_err(|e| ApiError::InternalServerError(e.description().to_string()))?;
    Ok(web::Json(result))
}

pub async fn update(
    update: web::Json<api::UpdateRequest>,
    state: web::Data<Arc<Mutex<AppState>>>,
) -> Result<web::Json<api::Status>, ApiError> {
    let mut state = state.lock().unwrap();
    state.update_refs = update.0.refs.clone();

    let result = state
        .receiver
        .check_update(update.0.refs)
        .map_err(|e| ApiError::InternalServerError(e.description().to_string()))?;
    Ok(web::Json(result))
}

pub async fn objects(
    objects: web::Json<api::MissingObjectsArgs>,
    state: web::Data<Arc<Mutex<AppState>>>,
) -> Result<web::Json<api::MissingObjectsResponse>, ApiError> {
    let state = state.lock().unwrap();
    let mut missing = vec![];

    for object in objects.0.wanted {
        let temp_path = state.receiver.temp_path(&object.object_name);
        let obj_path = state.receiver.obj_path(&object.object_name);

        if temp_path.exists() {
            let checksum = calculate_checksum(&temp_path)
                .map_err(|e| ApiError::InternalServerError(e.description().to_string()))?;
            if object.checksum != checksum {
                missing.push(object.clone());
            }
        } else if obj_path.exists() {
            let checksum = calculate_checksum(&obj_path)
                .map_err(|e| ApiError::InternalServerError(e.description().to_string()))?;
            if object.checksum != checksum {
                missing.push(object.clone());
            }
        } else {
            missing.push(object.clone());
        }
    }

    Ok(web::Json(api::MissingObjectsResponse { missing: missing }))
}

pub async fn upload(
    mut payload: Multipart,
    state: web::Data<Arc<Mutex<AppState>>>,
) -> Result<web::Json<api::Status>, ApiError> {
    let mut state = state.lock().unwrap();

    let mut rev = "".to_string();
    let mut object_name = "".to_string();
    let mut checksum = "".to_string();

    while let Some(item) = payload.next().await {
        let mut field = item.map_err(|e| ApiError::InternalServerError(format!("{}", e)))?;
        let content_type = field.content_disposition().unwrap();
        let name = match content_type.get_name() {
            Some(name) => name,
            None => {
                continue;
            }
        };

        if content_type.get_filename().is_some() {
            if rev.len() == 0 || object_name.len() == 0 || checksum.len() == 0 {
                continue;
            }

            let mut receive = false;
            let temp_path = state.receiver.temp_path(&object_name);
            let obj_path = state.receiver.obj_path(&object_name);

            // Receive the object if it doesn't exist or it's corrupt or incomplete
            if temp_path.exists() {
                let old_checksum = calculate_checksum(&temp_path)
                    .map_err(|e| ApiError::InternalServerError(e.description().to_string()))?;
                if checksum == old_checksum {
                    debug!("Object {} previously received", &object_name);
                    state.received_objects.push(object_name.to_owned());
                    return Ok(web::Json(api::Status {
                        status: true,
                        message: Some(format!("Object {} previously received", &object_name)),
                    }));
                } else {
                    receive = true;
                }
            } else if obj_path.exists() {
                let old_checksum = calculate_checksum(&temp_path)
                    .map_err(|e| ApiError::InternalServerError(e.description().to_string()))?;
                if checksum == old_checksum {
                    debug!("Object {} already stored", &object_name);
                    return Ok(web::Json(api::Status {
                        status: true,
                        message: Some(format!("Object {} already stored", &object_name)),
                    }));
                } else {
                    receive = true;
                }
            }
            if receive {
                // File system operations are blocking, we have to use threadpool
                debug!("Receiving object {}", &object_name);
                let mut f = web::block(|| std::fs::File::create(temp_path))
                    .await
                    .map_err(|e| {
                        ApiError::InternalServerError(format!("Failed to create file: {}", e))
                    })?;
                while let Some(chunk) = field.next().await {
                    let data = chunk.unwrap();
                    f = web::block(move || f.write_all(&data).map(|_| f))
                        .await
                        .map_err(|e| {
                            ApiError::InternalServerError(format!("Failed to write file: {}", e))
                        })?;
                }
                debug!("Object {} received", &object_name);
                state.received_objects.push(object_name.to_owned());
            }
        } else {
            // Values
            while let Some(value) = field.next().await {
                let data: actix_web::web::Bytes =
                    value.map_err(|e| ApiError::InternalServerError(format!("{}", e)))?;
                match name {
                    "rev" => unsafe { rev.push_str(std::str::from_utf8_unchecked(&data)) },
                    "object_name" => unsafe {
                        object_name.push_str(std::str::from_utf8_unchecked(&data))
                    },
                    "checksum" => unsafe {
                        checksum.push_str(std::str::from_utf8_unchecked(&data))
                    },
                    _ => {}
                }
            }
        }
    }

    Ok(web::Json(api::Status {
        status: true,
        message: None,
    }))
}

pub async fn done(
    state: web::Data<Arc<Mutex<AppState>>>,
) -> Result<web::Json<api::Status>, ApiError> {
    let mut state = state.lock().unwrap();

    // Move all received objects
    info!("Publishing {} objects...", &state.received_objects.len());
    for filename in &state.received_objects {
        let temp_path = state.receiver.temp_path(&filename);
        let obj_path = state.receiver.obj_path(&filename);
        let parent_path = obj_path.parent().unwrap();
        debug!("Create {:?}", &parent_path);
        fs::create_dir_all(&parent_path).map_err(|e| {
            ApiError::InternalServerError(format!("Failed to create object directory: {}", e))
        })?;
        debug!("Move {:?} to {:?}", &temp_path, &obj_path);
        fs::rename(&temp_path, &obj_path).map_err(|e| {
            ApiError::InternalServerError(format!(
                "Failed to move object inside the repository: {}",
                e
            ))
        })?;
    }
    state.received_objects.clear();

    // Update refs and generate delta
    state
        .receiver
        .update_refs(state.update_refs.clone())
        .map_err(|e| ApiError::InternalServerError(e.description().to_string()))?;
    state.update_refs.clear();

    Ok(web::Json(api::Status {
        status: true,
        message: None,
    }))
}
