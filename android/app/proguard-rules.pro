# Keep Moshi-serialized models stable when shrinking.
-keep class com.moi.lumine.model.** { *; }
-keep @com.squareup.moshi.JsonClass class * { *; }
-keep class **JsonAdapter { *; }

# Preserve metadata used by Moshi and Kotlin reflection.
-keepattributes Signature
-keepattributes RuntimeVisibleAnnotations
-keepattributes RuntimeVisibleParameterAnnotations
-keepattributes AnnotationDefault

# Keep the mobile binding entry points reachable from the app.
-keep class mobile.** { *; }
