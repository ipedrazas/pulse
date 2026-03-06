fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(false) // agent is a client only
        .compile_protos(&["../proto/pulse/v1/pulse.proto"], &["../proto"])?;
    Ok(())
}
