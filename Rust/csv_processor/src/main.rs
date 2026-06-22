use std::fs::File;
use std::io::{BufRead, BufReader};
use anyhow::Result;

fn main() -> Result<()> {
    let file = File::open("sample.csv")?;
    let reader = BufReader::new(file);

    let mut all_first_fields: Vec<String> = Vec::new();

    for line in reader.lines() {
        let line: String = line?;
        let fields: Vec<&str> = line.split(',').collect();
        all_first_fields.push(fields[0].to_string());
    }

    println!("{:?}", all_first_fields);
    Ok(())
}
