/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

use actix_web::{error::ResponseError, http::StatusCode, HttpResponse};
use failure::Fail;
use std::error::Error;
use std::fmt;

// ApiError

#[derive(Fail, Debug)]
pub enum ApiError {
    #[fail(display = "Internal Server Error ({})", _0)]
    InternalServerError(String),

    #[fail(display = "NotFound")]
    NotFound,

    #[fail(display = "BadRequest: {}", _0)]
    BadRequest(String),

    #[fail(display = "WrongRepoState({}): {}", _2, _0)]
    WrongRepoState(String, String, String),

    #[fail(display = "WrongPublishedState({}): {}", _2, _0)]
    WrongPublishedState(String, String, String),

    #[fail(display = "InvalidToken: {}", _0)]
    InvalidToken(String),

    #[fail(display = "NotEnoughPermissions")]
    NotEnoughPermissions(String),
}

impl ApiError {
    pub fn to_json(&self) -> serde_json::Value {
        match *self {
            ApiError::InternalServerError(ref message) => json!({
                "status": 500,
                "error-type": "internal-error",
                "message": message,
            }),
            ApiError::NotFound => json!({
                "status": 404,
                "error-type": "not-found",
                "message": "Not found",
            }),
            ApiError::BadRequest(ref message) => json!({
                "status": 400,
                "error-type": "generic-error",
                "message": message,
            }),
            ApiError::WrongRepoState(ref message, ref expected, ref state) => json!({
                "status": 400,
                "message": message,
                "error-type": "wrong-repo-state",
                "current-state": state,
                "expected-state": expected,
            }),
            ApiError::WrongPublishedState(ref message, ref expected, ref state) => json!({
                "status": 400,
                "message": message,
                "error-type": "wrong-published-state",
                "current-state": state,
                "expected-state": expected,
            }),
            ApiError::InvalidToken(ref message) => json!({
                "status": 401,
                "error-type": "invalid-token",
                "message": message,
            }),
            ApiError::NotEnoughPermissions(ref message) => json!({
                "status": 403,
                "error-type": "token-insufficient",
                "message": format!("Not enough permissions: {}", message),
            }),
        }
    }

    pub fn status_code(&self) -> StatusCode {
        match *self {
            ApiError::InternalServerError(ref _internal_message) => {
                StatusCode::INTERNAL_SERVER_ERROR
            }
            ApiError::NotFound => StatusCode::NOT_FOUND,
            ApiError::BadRequest(ref _message) => StatusCode::BAD_REQUEST,
            ApiError::WrongRepoState(_, _, _) => StatusCode::BAD_REQUEST,
            ApiError::WrongPublishedState(_, _, _) => StatusCode::BAD_REQUEST,
            ApiError::InvalidToken(_) => StatusCode::UNAUTHORIZED,
            ApiError::NotEnoughPermissions(ref _message) => StatusCode::FORBIDDEN,
        }
    }
}

impl ResponseError for ApiError {
    fn error_response(&self) -> HttpResponse {
        if let ApiError::InternalServerError(internal_message) = self {
            error!("Responding with internal error: {}", internal_message);
        }
        if let ApiError::NotEnoughPermissions(internal_message) = self {
            error!(
                "Responding with NotEnoughPermissions error: {}",
                internal_message
            );
        }
        HttpResponse::build(self.status_code()).json(self.to_json())
    }
}

// GenericError

#[derive(Debug)]
pub struct GenericError(String);

impl GenericError {
    pub fn new(msg: &str) -> GenericError {
        GenericError(msg.to_string())
    }
}

impl fmt::Display for GenericError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Error for GenericError {
    fn description(&self) -> &str {
        &self.0
    }
}
