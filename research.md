# HealthTrack+ and similar platforms

Date: 17 November 2026. Figures come from official pages and store listings checked on 15 November 2026, plus this repo. Where two official pages disagree, both numbers are kept. A number that was not on a primary page is omitted.

Twenty-eight research lanes finished. Cult.fit, Ultrahuman, Strava, and the open personal-health-record lane (Fasten / Tidepool) did not return a full brief. Fasten and Tidepool appear only to the extent their source files were read directly.

No application code was changed for this file.

---

## Short version

HealthTrack+ is a patient’s own notebook: blood pressure, blood sugar, weight, medicines, prescriptions, mood, lifestyle, insurance, doctor appointments, and provider service requests in one account. No single platform in this set covers that whole list for India.

The products that are ahead fall into three groups:

- Watches and rings that collect sleep, heart rate, and oxygen on their own. Apple Health, Samsung Health, Google Health (formerly Fitbit), Garmin, WHOOP, Oura, Withings.
- India marketplaces that book medicines, labs, and doctors. Practo, Tata 1mg, Apollo 24|7, PharmEasy, MediBuddy.
- ABDM, the national health-record rail. eka.care is on that rail. Samsung’s India records also open an ABHA through Eka.

The current gap is inside the notebook. Lifestyle, mental health, and insurance “add” buttons fire a `Feature coming soon` alert. Medicine frequency is `once` / `twice` / `thrice`. There is no clock time, alarm, or taken log. The chatbot has no authentication, and it must stay a general assistant.

The opening is a household timeline that can hold any pharmacy, any hospital, and any insurer, without a watch and without a shop cart. The product can call itself ABDM-compliant, a certified PHR, or a health locker only after a server-side P1–P3 integration and a directory listing.

---

## What HealthTrack+ stores today

Backend is Go. Roles are patient, doctor, provider, and admin. Auth is email OTP, Google login, and password reset. The chatbot `POST /chatbot/api` and TTS are separate and do not require login.

| Module | In the database | On the screen |
| --- | --- | --- |
| Vitals | Systolic, diastolic, blood sugar, weight, heart rate, temperature, oxygen, notes | Can be added on Health Track |
| Medicines | Name, dosage, frequency, start, end, prescribed by, notes, active | Can be added. No clock time or reminder |
| Prescriptions | Doctor, hospital, diagnosis, date, follow-up, document, notes | Can be added |
| Mental health | Mood, stress, sleep hours, sleep quality, anxiety, notes | List exists. Add button says “coming soon” |
| Lifestyle | Water, exercise minutes, steps, calories in, calories burned, smoking, alcohol, notes | List exists. Log Activity says “coming soon” |
| Insurance | Type, provider, policy number, coverage, premium, start, end, document, active | List exists. Add Policy says “coming soon”. “Expiring Soon” card shows `--` |
| Appointments | Patient, doctor, date, time, reason, status, type, meeting link | Doctor can accept, reject, or complete |
| Services | Provider service, price, patient request | Accept, decline, or complete |
| Admin | Users, approval | Health data, reports, and settings say “backend support coming soon” |

Daily blood pressure, mood, and policy rows are this app’s own log. They are separate from an ABDM care context.

---

## At a glance

Prices are from pages checked on 15 November 2026. India prices appear only where an India listing or an official India page printed the figure.

| Platform | What it is | India | Price on the page | What it has beyond HealthTrack |
| --- | --- | --- | --- | --- |
| Apple Health | Phone record | App exists. Several clinical features are US-only | No separate Health app price found | Watch, medicine reminders, PDF |
| Health Connect | On-device Android store | On Play | No price page | Steps, sugar, blood pressure from other apps |
| Samsung Health | Watch plus app | Yes, with ABHA | App is free. Subscription price not verified | Sensors, medicine interactions, hospital records |
| Google Health (Fitbit) | Watch-first | Premium ₹499/month or ₹4,999/year (India post, 7 May 2026) | US $9.99/month or $99/year | Sleep stages, Gemini coach |
| Garmin Connect | Watch log | App exists. No India Connect+ price | US Connect+ $6.99/month or $69.99/year. App is free | Steps, Pulse Ox, coach plans |
| WHOOP | Band membership | One ₹21,990, Peak ₹28,990, 12 months included. Life is not sold in India | US One $199, Peak $239, Life $359 per year | Recovery, strain, journal |
| Oura | Ring | Amazon and Croma. Membership via UPI | US ring $349–$499, membership $5.99/month or $69.99/year. No official India membership price | Sleep, readiness, cycle |
| Withings | Scale, cuff, watch | Hardware in INR. India is absent from the ScanWatch 2 ECG/SpO2 region list | Withings+ US $9.95/month or $99.50/year. One India page: ₹1,000/month or ₹9,500/year | Cuff blood pressure, CSV |
| MacroFactor | Diet coach | App shows ₹1,049/month. India annual price not verified | US $11.99/month, $47.99/6 months, $71.99/year | Weekly calorie target |
| MyFitnessPal | Food diary | App exists. Coach is not offered in India | Premium $79.99/year. Premium+ $99.99/year or $24.99/month | Barcode, claim of 20.5 million foods |
| Cronometer | Nutrient diary | Gold ₹999/month, ₹5,900/year | Basic $0. Gold $10.99/month or $59.99/year | 95 nutrients, verified database |
| Noom | Habit course | App exists. Clinic is US-only | Weight $70/1 month through $209/12 months | Daily lessons |
| HealthifyMe | Indian calories | Built here | Pages disagree. Healthify+ store ₹399/1 month, ₹1,499/12 months. Rx from ₹9,999 | Indian foods, Ria |
| Bearable | Symptoms and mood | App, English only | FAQ says $34.99. India lifetime ₹9,900 | Correlations, experiments |
| Levels | US CGM wellness | Not available | Membership $15/month or $80/year | Glucose curve next to a meal |
| Function Health | Lab membership | Not available | $365/year. Family $340 | 160+ biomarkers, clinician notes |
| Medisafe | Dose alarms | Locks after 14 days, then paid | $4.99/month or $39.99/year. India store monthly ₹299 and ₹499, yearly ₹2,499–₹3,999 | Clock time, refill, Medfriend |
| Ada | Symptom check | App is free. No Hindi | No consumer price. Free | Probability report. Leave it out of HealthTrack |
| MyChart | Hospital portal | No Indian hospital found on a primary source | No patient subscription | Labs, eCheck-In, proxy |
| Teladoc | US virtual care | Not an India consumer doctor service | Billing FAQ: 24/7 care $89. Therapy page: $95. The two pages disagree | Price before confirm, visit note |
| Practo | Doctors and clinics | Built here | Audio from ₹199. Plus ₹1,199/30 days | Directory, prescriptions, Ray |
| eka.care | PHR plus EMR | Built here, with ABHA | Patient app free. Eka Pass ₹49/month, ₹449/year. Doctor Plus ₹16,999/year | ABHA, Gmail, scribe |
| Tata 1mg | Pharmacy and labs | Built here | Example: Maglectin ₹249. Gold package ₹2,499 | Delivery, reports |
| Apollo 24\|7 | Hospital storefront | Built here | Consults from ₹200. Circle blog ₹199/12 months (12 May 2025) | Records from Apollo hospitals |
| PharmEasy | Pharmacy and labs | Built here | Consults from ₹249. No single Plus price | Order from a prescription |
| MediBuddy | OPD wallet | Built here | Browse free. Consults are pay-per-use | Cashless label, ABHA |
| ABDM | Public rail | This is the rail | No separate citizen app price | Linked hospital records |

---

## Phone health layer

### Apple Health

Apple’s Health app runs on iPhone, iPad, and Watch. No Android or web app was found. An India site exists. Hindi is absent from the 3 November 2026 language list.

Blood pressure logging covers 7 days of morning and evening readings, or 28 daily readings, plus reminders and a PDF. Classifications shown are AHA, ESC, or FIGO. Blood sugar can be typed into HealthKit. Heart rate, ECG (age 22 or older, on specific watches), and Blood Oxygen are wellness measurements. Medicines have a list, a schedule, a follow-up 30 minutes later, taken or skipped, and a PDF. Camera label scan and drug-interaction checks are documented as United States only. In India the name is typed.

Hypertension notifications disagree across Apple’s own pages. India support (26 September 2026) calls the feature available. The India Health page fetched the same day did not show that section. That support article also said Series 12 and Ultra 4 clearance was still in process.

Worth copying: a medicine PDF, cuff-log reminders, and sharing. Leave out a design that only works with a watch, and leave the US interaction checker off free-text Indian medicine names.

Sources: [apple.com/in/health](https://www.apple.com/in/health/), [Blood Oxygen](https://support.apple.com/en-in/120358), HealthKit docs.

### Google Health Connect

This is an on-device Android database, included in Settings on Android 14. There is no iOS or browser client. Medical records cannot be exported. Google Fit APIs run until the end of 2026. New developer sign-ups have been closed since 1 May 2024.

It can hold blood pressure, glucose, heart rate, oxygen, temperature, weight, hydration, calories, and FHIR medications. Play policy forbids using this data to decide insurance eligibility.

A later HealthTrack pipe can read it with a fixed order: a manual entry first, then Health Connect. The notebook still works for someone without a watch.

Sources: [Health Connect](https://developer.android.com/health-and-fitness/health-connect), [Fit migration](https://developer.android.com/health-and-fitness/health-connect/migration/fit).

### Samsung Health

Free in India. Blood pressure (recalibrate every 28 days) and ECG run on Galaxy Watch4 and later, paired with a Galaxy phone. Samsung says the blood-pressure app is not for diagnosis and should not be used by people with diabetes. No India page describing blood sugar was found.

The app has a medication list, reminders, and interaction warnings. A scanned prescription or pill packet lands in that list. Samsung Newsroom India, 17 January 2025: create or open an ABHA in the app with Aadhaar or a mobile number, via Eka Care. 19 February 2026: Find Care with PharmEasy and Tata 1mg, including six complimentary months of PharmEasy Plus. The paid price of that Plus plan was not verified.

Sources: [Samsung India](https://www.samsung.com/in/apps/samsung-health/), [ABHA newsroom](https://news.samsung.com/in/samsung-introduces-personal-health-records-feature-on-samsung-health-app-in-india).

### Google Health, formerly Fitbit

On 19 May 2026 the Fitbit app became the Google Health app. Fitbit accounts ended. Premium is now Google Health Premium. The coach, sleep stages, and Fitness tab still require a paired Fitbit or Pixel Watch. Without one, the app shows Today and Health.

India, Google post of 7 May 2026: ₹499 per month or ₹4,999 per year. Fitbit Air reached Indian shelves on 2 October 2026 at ₹13,999. The US help article says it does not apply to India, so the India price is the India post.

Medications arrive through a US provider connection or Health Connect. Glucose can be typed. Blood pressure is absent from the manual metric list.

Sources: [India coach post](https://blog.google/intl/en-in/products/hardware/google-health-coach-is-coming-to-india/), [healthapp.google India](https://healthapp.google/intl/en_in/your-health-connected/).

---

## Food, weight, and habits

These products sit next to HealthTrack’s lifestyle fields. HealthTrack stores calories as one number. The Log Activity button still alerts.

### Cronometer

Basic is free. Gold is $10.99 per month or $59.99 per year. India store: Gold ₹999 per month, ₹5,900 per year. Up to 95 nutrients, and over 1 million lab-analyzed foods from USDA and the Nutrition Coordinating Center. History is 7 days on Basic and any timeframe on Gold. Crono Coach, 28 July 2026, answers from the diary, does not edit it, and is not allowed to change medications. No official claim of an Indian food database.

Worth copying: foods logged as servings. Leave ads off a clinical vault, and leave medicine history on the full timeline.

Source: [cronometer.com/gold](https://cronometer.com/gold/).

### MyFitnessPal

Premium $79.99 per year. Premium+ $99.99 per year or $24.99 per month. Barcode scan has been Premium-only since 1 October 2022. Nutrition Coach is iOS, English, and six countries. India is not on that list. The India listing claims 20.5 million foods. The database is community-built. In 2018 an unauthorized party acquired usernames, email addresses, and hashed passwords. The company FAQ says later alerts refer to that same incident. No prescriptions, insurance, or appointments. GLP-1 logging is United States and Canada.

Worth copying: barcode and photo, confirmed by the person before the log. Keep export on the free record. Keep the diary free of ads. Keep deficit math as one lifestyle field.

Sources: [Premium](https://www.myfitnesspal.com/premium), [privacy](https://www.myfitnesspal.com/privacy-policy).

### MacroFactor

A diet coach. US prices: $11.99 per month, $47.99 for six months, $71.99 per year. No free tier. A 7-day trial. The India app shows a monthly plan at ₹1,049. The India annual price was not verified. They describe expenditure as a deterministic calculation from logged intake and the weight trend. A partial day is treated as the full intake. Heart rate from integrations is displayed and not stored. No medicines.

Worth copying: a weekly target from the weight trend and actual intake, plus a manual mode. Keep photo calories and macro coaching beside the vital record.

Source: [macrofactor.com/macrofactor](https://macrofactor.com/macrofactor/).

### Noom

Weight plan, cost page updated 3 June 2026: $70 for 1 month through $209 for 12 months, billed upfront. No $0 trial is published. Examples on the page run from $0.50 to $18. Welli answers general questions. Medical questions stay off Welli. One-to-one coaching is a separate US Premium add-on. The GLP-1 clinic is US-only and not in every state. The privacy policy says the site may sell sensitive personal data. India App Store rupee rows do not show the billing period.

Worth copying: a 5–15 minute lesson on the same day as a log. Keep the record separate from a weight subscription and a medication offer.

Sources: [cost](https://www.noom.com/blog/how-much-does-noom-cost/), [clinic pricing](https://www.noom.com/med/pricing/).

### HealthifyMe

Bengaluru. The coach is still named Ria. The app page claims 10,000+ Indian foods. Other official lines disagree, so that claim stays separate. Prices on the same week disagree by page. Store Healthify+ is ₹399 for 1 month (struck-through ₹799) and ₹1,499 for 12 months. Smart with a CGM is ₹6,000 for 14 days. Pro says “plans from ₹3,500/month”. Rx starts at ₹9,999, ₹20,999, and ₹30,999, with one month of Wegovy, Yurpeak, or Mounjaro if a doctor prescribes it. The privacy policy changed 28 October 2026 says food photos, glucose, and biomarkers may be de-identified and licensed, including for model training.

Worth copying: search over Indian foods, and one answer from that day’s logs. Keep Snap opt-in. Leave the coach, CGM, and GLP-1 price ladder out of the medical record.

Sources: [healthifyme.com/in](https://www.healthifyme.com/in/), [store](https://store.healthifyme.com/products/healthify-plan-your-24-7-ai-health-coach), [Rx](https://rx.healthify.com/pricing).

### Bearable

Mood, sleep, medicines, symptoms, and factors, then correlations. The free tier includes unlimited logs and CSV. There is no in-app model. The site tells people to export and open ChatGPT. The FAQ states $34.99 and a 7-day trial. A page dated 13 October 2020 still says $6.99 per month. The India store lists many SKUs, including lifetime at ₹9,900. There is no doctor login, no Hindi, no web app. Health data is in Frankfurt.

Worth copying: a free core, experiments (one factor, a duration, up to eight outcomes), and a way to hide calories. A CSV is an export. A doctor, provider, or insurer relationship is a login with a permission.

Source: [bearable.app](https://bearable.app/).

---

## Watches, rings, and labs

### Garmin Connect

The app and the history that existed before Connect+ stay free. US Connect+ is $6.99 per month or $69.99 per year (press release, 27 March 2025). Japan is ¥1,180 / ¥11,800. No official India Connect+ price was found. A forum post of $90 is not an official price. Index S2 was listed in India at ₹19,990 and out of stock that day. Venu X1 is ₹97,990. Pulse Ox is not a medical device and is not offered in every country. Garmin Coach is free. Active Intelligence is Connect+ only.

Sources: [Connect+](https://www.garmin.co.jp/products/subscription-plans/garmin-connect-plus-monthly/), [garmin-india.com](https://www.garmin-india.com/).

### WHOOP

India: One ₹21,990 and Peak ₹28,990, taxes included, each with 12 months and a WHOOP 5.0. Life / MG is marked not available in India, so ECG and blood-pressure insights are absent from the India lineup. US: One $199, Peak $239, Life $359 per year. Recovery, Strain on a 0–21 scale, and four sleep stages. The journal lists 300+ behaviors. An association appears after 5 yes and 5 no within 90 days. Glucose and water glasses are absent from the membership lineup.

Sources: [whoop.in](https://www.whoop.in/pages/whoop-devices), [US membership](https://www.whoop.com/us/en/membership/).

### Oura

India is a retail market, not direct shipping (help article, 7 October 2026). Membership uses UPI. Credit cards were temporarily unavailable as of 3 November 2026. US Ring 5 is $399 or $499. US membership is $5.99 per month or $69.99 per year. No official INR membership figure is on Oura’s price card. Press reports said ₹599 per month. No annual INR price was on Oura’s help pages. Without a membership the app shows three scores. Heart-rate, SpO2, and temperature detail stay locked. CSV export still works after the plan ends. Advisor, Cycle Insights, Pregnancy Insights, and Menopause Insights are on Gen3 and newer with membership. Oura says the ring is not a medical device. Fertile Window is a conception aid and is not contraception.

Worth copying: that plain line next to any score, and export that survives a cancelled plan. A readiness score needs overnight heart rate, HRV, and temperature. Tags for caffeine, alcohol, and mood can sit beside typed vitals.

Sources: [membership](https://ouraring.com/membership), [support](https://support.ouraring.com/hc/en-us/articles/4409086524819-Oura-Membership).

### Withings

The app is free. Measurements, PDF, CSV, and HealthLink stay free. Withings+ in the US is $9.95 per month or $99.50 per year. The BodyScan 2 India page shows ₹1,000 per month or ₹9,500 per year. Accounts created after 13 October 2026 pay to sync out to other apps. ScanWatch 2 is sold in India. India is absent from the ECG and SpO2 region matrix. A cuff with a three-reading sequence is the blood pressure. A scale hypertension notification is a pattern score. Wrist temperature deviation is a baseline, and BeamO is the contactless thermometer. Withings does not sell a fingerstick glucometer.

Worth copying: free measurements and a free clinician PDF. Say which of ECG, SpO2, and cuff pressure work in India on the product page.

Sources: [Withings+](https://www.withings.com/en-us/pages/withings-plus), [ScanWatch 2 regions](https://support.withings.com/hc/en-us/articles/17091438334353-ScanWatch-2-Feature-Availability-By-Region).

### Levels

United States only, availability page updated 28 July 2026. Membership is $15 per month or $80 per year. The app runs without a sensor. Stelo is $89 for two sensors. Basic labs are $99. Comprehensive labs are $399. Levels says it is not a healthcare provider. Its non-diabetic bands, including a fasting example of 80–86 mg/dL, stay off the default targets of an India blood-sugar log.

Worth copying: typed readings that are useful on their own, with sensors optional, and meal or sleep context beside the number.

Source: [levels.com](https://www.levels.com/).

### Function Health

United States, Quest. $365 per year, down from $499. Family add-on $340. Two lab rounds a year, marketed as 160+ tests. No India service. A person calls on a critical result. The member chat says it does not give medical advice. New York and New Jersey can cost more. No dollar amount is published. MRI prices on Function’s own pages include $499, $899, and “starting at $365”, so none is treated as the single current price.

Worth copying: one line on every saved reading, why it matters, and one next step, plus a PDF for a doctor. Keep paid add-ons outside that explanation.

Source: [functionhealth.com](https://www.functionhealth.com/).

---

## Medicines and symptoms

### Medisafe

From 1 January 2026, outside the United States the app locks after a 14-day trial unless the user subscribes. Help center: $4.99 per month or $39.99 per year. India App Store SKUs include monthly ₹299 and ₹499, and yearly ₹2,499, ₹2,999, and ₹3,999. The interaction checker is US and English. Medisafe does not order medicines. A Medfriend can view and cannot edit. A missed-dose alert fires about one hour after the scheduled time. The privacy policy allows sharing with pharmaceutical partners. The marketing line that they never share does not match that policy.

HealthTrack’s frequency label does not notify anyone.

Worth copying: clock time, taken, skipped, and snooze. A refill countdown that stops short of placing an order. A view-only caregiver. Keep the medicine list available after any trial, and keep a US interaction checker off free-text Indian names.

Sources: [paid-app note](https://help-center.medisafe.com/en/articles/13052285-medisafe-is-now-a-fully-paid-app-for-users-outside-the-u-s), [price](https://help-center.medisafe.com/en/articles/8102603-how-much-does-medisafe-premium-cost).

### Ada

The consumer assessment is free. It returns possible causes and how likely Ada considers them. Ada says that is not a diagnosis. The library is curated by clinicians. Ada’s own medical-quality page says Ada Assess is a Class IIa device under EU rules, CE 0123. No FDA or CDSCO page naming Ada was retrieved, so no US or Indian device status is stated here. Hindi is absent. It was still absent after languages were restored on 24 June 2026.

HealthTrack’s chatbot stays off that list: possible conditions, likelihoods, “pre-diagnosis”, an acuity score, and “certified medical device”. If someone asks what condition they have, or describes an emergency, stop the reply and show 112 and 108, plus a prompt to see a clinician.

Sources: [ada.com](https://ada.com/), [diagnosis help](https://ada.com/help/can-ada-give-me-a-medical-diagnosis-can-ada-diagnose-medical-conditions/).

---

## Hospitals and virtual care

### MyChart

Epic’s patient portal. It works for someone who already has an account at an organization that runs Epic. No primary source in this search named an Indian hospital on MyChart. The patient app is free. It can show labs, visit summaries, medication renewal, eCheck-In, video visits, proxy access, and bills, when that organization turns them on. Art is for clinicians. Emmie writes patient-facing summaries. Happy Together can combine participating organizations. Share Everywhere gives a temporary browser code.

Worth copying: who opened the record, a trend PDF, and a clear proxy rule. A personal notebook stays labelled as the person’s log until it is wired to a hospital chart.

Sources: [mychart.org](https://www.mychart.org/), [Emmie](https://www.epic.com/software/emmie/).

### Teladoc

United States and Canada. The consumer product is not an India doctor service. Global support lists +91 (0)11 7127 9637. Separate sponsor apps exist. Prices disagree across Teladoc’s own pages. The billing FAQ lists 24/7 care at $89 and mental health at $119 or more. The mental-health page lists therapy at $95, initial psychiatry at $235, and psychiatry follow-up at $105. Cancelling inside 24 hours can cost up to $50. The account shows the cost before confirm. A summary lands in the secure message center afterward. Urgent care starts after symptom intake, without a doctor accept button.

Worth copying: the fee before confirm, the late-cancel rule, and a short note after the visit. Video belongs in the product. Prescribing stays with a clinician.

Sources: [teladochealth.com](https://www.teladochealth.com/), [billing FAQ](https://www.teladochealth.com/helpcenter/article/billing-and-insurance-faqs).

---

## India marketplaces and records

### Practo

Patients find doctors, book clinic visits, consult online, and order medicines and tests. Clinics run on Ray, hospitals on Insta, queues on Qikwell. Private audio “starts at just ₹199”, with a 7-day free follow-up. Specialty prices on the same page run from ₹700 to ₹899. Prime booking is free for the patient. Plus is ₹1,199 every 30 days (15 consults a month), ₹2,499 every 3 months, or ₹2,999 a year. The about page still says “unlimited” with a fair-usage asterisk. The priced page, with caps, is the one to use. Ray’s plan page lists 999 and 1,499 per month and does not print a currency symbol. Practo’s blog says Ray joined ABDM.

HealthTrack has a doctor queue. It does not have a city directory, a council check, or a digital prescription from the visit.

Worth copying: free booking, with the fee set by the doctor, and the follow-up window written down. Practitioner profiles stay with the practitioner. A guide in the product stays information.

Sources: [consult](https://www.practo.com/consult), [Plus](https://www.practo.com/plus), [Ray](https://www.practo.com/providers/clinics/ray).

### eka.care

Orbi Health. A patient PHR, ABHA, a doctor EMR, and EkaScribe. The National Health Authority partner page lists “Eka Care EMR”. The patient app is free. Eka Pass is ₹49 per month and ₹449 per year. No Eka page fetched here states which features that pass unlocks. Doctor plans, exclusive of 18% GST: Doc Plus ₹16,999 per year, Doc Pro ₹18,749, Clinic Pro ₹1,00,000. From 1 September 2025 the free doctor plan is limited to 30 days of patient data, 1 GB, and one seat. No later official reversal was found.

Patients can upload files, turn lab reports into structured data, keep family profiles, store a Secret Locker on the device, connect Gmail read-only, and use ABHA Scan and Share. Doctors on the paid plan can send a digital prescription. EkaScribe can draft one from consultation audio, and WhatsApp is a delivery channel.

This is the closest full India platform. The remaining opening is daily blood pressure, mood, and insurance together, in a small household log, without a ₹17,000 doctor seat.

Sources: [eka.care](https://www.eka.care/), [plans](https://www.eka.care/practice-management/plan-pricing), [ABDM Connect](https://developer.eka.care/docs/hie-cm/phr/overview).

### Tata 1mg

Tata Digital. Medicines, home labs, and online consults. The download page says online consults are free. No public consult fee card was on the pages fetched. A live Maglectin page showed ₹249 against an MRP of ₹285. The homepage listed Comprehensive Gold Full Body Checkup at ₹2,499 against ₹4,998. The download page says home sample collection is free. Health records, family records, and a pill reminder appear on the product. Pulse, described on the about page, is an AI product for pharma companies.

Worth copying: the prescription file kept on the medicine it authorizes. A medicine from any pharmacy can sit in the log without a checkout.

Sources: [1mg.com](https://www.1mg.com/), [privacy](https://www.1mg.com/PrivacyPolicy).

### Apollo 24|7

Apollo Healthco. Online consults start at ₹200. A cancelled online consult is refunded in 5–7 working days. A blog updated 12 May 2025 prices Circle at ₹199 for 12 months. That figure is the article, not a live cart on 15 November 2026. The national CBC page shows ₹449. The Delhi page shows ₹399 in the hero and ₹430 in the city table. Apollo Full Body Checkup – Basic shows ₹879. Doctor counts disagree: 6,000+ on About, 4,400+ on Play, 2,200+ on the App Store. Hospital records sync from Apollo’s own systems. The insurance page headlines “₹1 crore cover at ₹1.5 per day” with an asterisk and no age or deductible, so it is not used as a price. Apollo 24|7 is absent from the PHR apps named in the ABDM FAQ. Apollo Hospitals had completed three ABDM milestones as of 30 June 2023. That note is about the hospitals.

Worth copying: prescriptions and reports kept on one person. Records from other hospitals stay in the same timeline.

Source: [apollo247.com](https://www.apollo247.com/).

### PharmEasy

Threpsi Solutions owns the platform. Axelia Solutions operates it. Threpsi is wholly owned by API Holdings. Medicine delivery is described as 19,000+ PIN codes in 1,200+ cities. The consult page says “starting at ₹249”. A blog updated 1 April 2026 still said ₹199. The consult FAQ says a consult is free for anyone who books a PharmEasy Labs test. Swasth.AI on WhatsApp explains lab reports and says it is not a diagnosis. The privacy policy allows prescriptions to be used for marketing and transferred outside India. Storage named there is AWS in India, Google Cloud in Singapore, and MongoDB Cloud in Ireland.

Worth copying: an uploaded prescription kept on the medicine, and a refill reminder kept separate from the dose schedule. Marketing stays off the prescription. Order history stays separate from the clinical entry.

Sources: [pharmeasy.in](https://pharmeasy.in/), [privacy](https://pharmeasy.in/legal/privacy-policy).

### MediBuddy

Phasorz Technologies. DocsApp was folded into this brand in 2020. Medi Assist, the listed TPA, is a separate company. Browsing is free, and a retail user can use the app with no employer. The OPD wallet exists only on a corporate or insurer plan. Consults, labs, and medicines are pay-per-use by UPI, debit card, or credit card. Mia is a chatbot with a notice that answers may be wrong. The privacy policy says data is hosted in India. An employer can receive data on a formal request, under the approvals the policy names.

Worth copying: a checkout label before confirm (wallet, co-pay, or self-pay), and book, pay, cancel, with the prescription kept on that person. The household policy list holds any insurer. A TPA claims desk stays with the TPA.

Sources: [medibuddy.in](https://www.medibuddy.in), [terms](https://www.medibuddy.in/terms.html).

---

## ABDM and ABHA

Public pages read on 15 November 2026. This is what those pages describe. It is not a legal opinion.

An ABHA number is a 14-digit identifier. An ABHA address looks like `name@abdm`. It is separate from an Ayushman card and from PM-JAY eligibility. Enrollment is voluntary. The FAQ says lack of an ABHA number is not a reason to deny treatment.

HIE-CM checks identity and consent, and only then allows records to move. Clinical records stay with the provider that created them. A hospital system is a HIP. The party that asks to read is an HIU. A locker subscribes for a notification. The notification does not contain the record. Consent comes first, then a health-information request.

A private app that wants to show linked records needs a server bridge for P1 (address), P2 (care context), and P3 (consent and fetch). The `clientSecret` stays off the browser. The path after that is sandbox access, Health Tech Committee review, a security audit, and a listing in the ABDM Application Directory. User-uploaded files need health-locker certification and M2. Appointments are the UHI rail. Claims are the NHCX rail. The HIE-CM facility page does not document an NHCX endpoint.

The UI can say the rows are the person’s HealthTrack log. It stays silent on ABDM-compliant, certified PHR, health locker, consent manager, “ABDM stores the records”, “an ABHA is an Ayushman card”, “a saved blood pressure is a national record”, and “another hospital can see this”.

Sources: [FAQ](https://abdm.gov.in/FAQ), [PHR](https://docs.abdm.gov.in/docs/hiecm/v3/concepts/participants/phr), [P1](https://docs.abdm.gov.in/docs/hiecm/v3/milestones/p1), [P3](https://docs.abdm.gov.in/docs/hiecm/v3/milestones/p3), [P4](https://docs.abdm.gov.in/docs/hiecm/v3/milestones/p4).

---

## Insurance

HealthTrack stores type, provider, number, coverage, premium, start, end, document, and active. The screen does not show the document. The Add button is unwired. Expiring Soon shows `--`.

The Policybazaar app (Play listing updated 6 November 2026) stores policies bought or managed there: download, share with family, renew, find network hospitals, and track claim status. Niva Bupa’s health card carries policy number, insured name, coverage, plan type, and validity. At a network hospital the card is shown with a photo ID. Niva’s FAQ limits its own app to the in-force period. A health record still keeps the expired policy beside the care history.

DigiLocker: IRDAI circular of 9 February 2021. United India and IFFCO-TOKIO document pulling a policy file into DigiLocker. UIIC’s steps use customer ID, effective date, policy number, and consent.

Worth copying: the file on the row, a shareable card, “expiring soon” from the end date, and a mark for whether the person typed the row or an insurer issued it. Settlement stays with the insurer.

Sources: [Niva card](https://www.nivabupa.com/health-insurance/health-card.html), [IRDAI circular](https://www.digilocker.gov.in/assets/img/Circulars/IRDAI%20-%20Circular%2009022021.pdf).

---

## Four lanes that did not finish

Cult.fit, Ultrahuman, and Strava have no prices or feature notes in this file. Those briefs did not come back.

Fasten Onprem’s README (the raw `LICENSE` URL returned 404; the license file is `LICENSE.md`): the open-source app cannot import from healthcare providers directly. A person enters data by hand, or uploads a FHIR bundle exported elsewhere. Hosted Fasten Connect is a separate service and is not in that repo.

The Tidepool platform repo uses a BSD-style license, copyright 2018, Tidepool Project. It is for diabetes device data. It can be read later next to HealthTrack’s blood-sugar field. It is not a spec to copy now.

---

## Where HealthTrack can win

The notebook already holds a combination the others split apart:

- Daily blood pressure and blood sugar, with no watch and no CGM purchase.
- A medicine’s start and end, plus a prescription file from any doctor.
- Mood, stress, and sleep hours.
- Every household policy, across insurers.
- A doctor who accepts the visit, and a separate provider service.
- Sarvam is already in the config, so speech can come out in Indian languages. Ada, Bearable, Cronometer, and Oura Advisor are English or European languages.

That holds while the log stays with the user, stays out of ads, and stays off a checkout. The privacy pages of PharmEasy, 1mg, Noom, and Healthify describe wider uses.

---

## Work that can be built now

No new fee and no certification. The screens already show the gaps.

1. Wire the add buttons for lifestyle, mental health, and insurance. The tables already exist.
2. Drive “expiring soon” from the policy end date, and show the document on the row.
3. Add clock time, taken, and skipped on medicines. Keep the frequency label.
4. One-click CSV for medicines, vitals, and prescriptions. Free.
5. One PDF for a doctor: latest blood pressure, latest sugar, active medicines, latest prescription.
6. On the chatbot, stop for emergencies and show 112 and 108.
7. Require login for chat, so the assistant can see the chart and the endpoint is not open.

Then, still without a sensor:

8. A refill count, and a caregiver who can view.
9. Sleep hours from the mental-health log on the dashboard. Sleep stages wait for a sensor.
10. If an appointment has a price, show it before confirm, with the cancel rule, and store a short note after the visit.
11. Optional Health Connect read. A manual entry wins.

ABDM, pharmacy delivery, a CGM, and watch scores come after this list. eka.care already offers the national rail. A locker claim waits on a server callback and a directory listing.

---

## Leave out

- Hiding sleep stages until a watch is purchased.
- Charging for barcode scan or for export.
- Treating a crowd-built calorie as a lab value.
- A weight-loss course or a GLP-1 offer inside the record.
- Remarketing from prescriptions.
- A US drug-interaction checker on Indian free text.
- Blocking the app when there is no ABHA.
- An “ABDM-compliant” badge.
- An AI reply that names a disease or directs a medicine.
