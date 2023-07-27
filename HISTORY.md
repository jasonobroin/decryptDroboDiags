# Genesis

I wrote an improved Drobo diag decrypting utility circa 2016 in the last few months of my time at Drobo.

There were a number of issues with the tooling prior to this

- A ziped log file was created, but it only had some of the useful files
- A number of files had to collected manually by the user, and support often had to have multiple conversations with customers to get everything needed, much to everyone's frustration
- The decrypt process was poor, and partially broken
- I don't think the zip or log files were timestamped 
- The main internal firmware log file was hard to look at as it was large with lots of sections
- Various useful internal information was not exposed

Historically, the overall custody chain of logs was bad - this project created a single Serial Number, time and date stamped zip archive which was easy for customers to send to us, could be analyed with much improved tooling, and could easily be uploaded (and downloaded) from our JIRA issue tracking system

In addition, I was looking for a meaningful project to learn golang (and also a bit of bootstrap and Javascript)

# Benefits

- All useful log files were collected and archived in one shot with tracability by Serial Number & collection timestamp
- A bug which occasionanly failed to decode part of the logs was found and fixed
- Many of the internal logs were well structured and it was possible to identify the different subsections, separate them out visually, allow automated linking to the different subsections, and allow open/colapse of different areas, and navigation within logs
- Decoders for various binary files were implemented allowing us to export internal data structures (such as the zone table) which historically had not been accessible without the return of a diskpack
- The firmware log generation process was overhauled to be more robust, load balance with other system activities and be able to generate previously unexportable diagnostic information
- Sets of logs could be much more easily handled, especially with the JIRA integation

# golang

As noted, the project was developed in golang - version 1.6 which was the most current at the time.

Pleasingly, [golang's compatibility promise](https://go.dev/doc/go1compat) holds up - to build with golang 1.20, I just needed to create a go.mod file and compilation just worked

# Encrypted logs

Drobo had encrypted its logs since the very early days - although there were probably some IP concerns, I recall it was largely an engineering level decision. Beyond RAID was extremely complicated due to its virtualization architecture, and we felt self-diagnosis from fairly arcane engineering level output was going to cause far more problems than it would solve. It often took US a lot of mental energy to interpret what was actually going on. It also allowed us to be MUCH more detailed in the internal telemetry we were generating than we could have been if they had been customer facing

Dashboard and NAS level logs were never encrypted

# Limitations

I don't believe Drobo's log files have materially changed since this utility was written - decoding and display seem to be working much as expected. 

The JIRA integration won't work correctly as it used an authentication model which has subsequently been deprecated, but its a moot point as Drobo is no more and JIRA integration won't be of any value

# Final notes

All the above is written from memory
