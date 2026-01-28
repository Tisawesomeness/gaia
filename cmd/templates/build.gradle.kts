plugins {
    id("java")
}

// TODO Replace with your own
group = "org.example"
version = "1.0-SNAPSHOT"

repositories {
    mavenCentral()
    maven {
        name = "hytale-release"
        url = uri("{{ .Repository }}/release")
    }
    maven {
        name = "hytale-pre-release"
        url = uri("{{ .Repository }}/pre-release")
    }
}

dependencies {
    compileOnly("{{ .Group }}:{{ .Artifact }}:{{ .Version }}")
}

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(25))
    }
}