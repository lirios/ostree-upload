// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

#pragma once

#include <glib.h>

static char *_g_error_get_message(GError *error) {
  g_assert(error != NULL);
  return error->message;
}

static gboolean _g_hash_table_iter_next_variant(GHashTableIter *iter,
                                                GVariant **key,
                                                GVariant **value) {
  g_assert(iter != NULL);
  return g_hash_table_iter_next(iter, (gpointer)key, (gpointer)value);
}

static void _g_variant_get_su(GVariant *v, const char **checksum,
                              OstreeObjectType *objectType) {
  g_assert(v != NULL);
  g_variant_get(v, "(su)", checksum, objectType);
}

static const char *_g_strdup(gpointer string) { return g_strdup(string); }
