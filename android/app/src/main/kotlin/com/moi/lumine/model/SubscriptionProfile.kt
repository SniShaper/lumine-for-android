package com.moi.lumine.model

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class SubscriptionProfile(
    @Json(name = "id") val id: String,
    @Json(name = "name") val name: String,
    @Json(name = "url") val url: String,
    @Json(name = "config_name") val configName: String,
    @Json(name = "updated_at") val updatedAt: Long = 0L
)
