package testdata

var TrackFixtures = []map[string]any{
	{"track_id": 100, "genre": "Electronic", "owner_id": 1, "title": "T1", "is_unlisted": "f", "is_downloadable": "t"},
	{"track_id": 101, "genre": "Alternative", "owner_id": 1, "title": "T2", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 200, "genre": "Electronic", "owner_id": 2, "title": "Culca Canyon", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 201, "genre": "Alternative", "owner_id": 2, "title": "Turkey Time DEMO", "is_unlisted": "t", "is_downloadable": "f"},
	{"track_id": 202, "genre": "Alternative", "owner_id": 2, "title": "Turkey Time (live)", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 300, "genre": "Electronic", "owner_id": 3, "title": "Follow Gated Download", "is_unlisted": "f",
		"download_conditions": []byte(`{"follow_user_id": 3}`), "is_downloadable": "t"},
	{"track_id": 301, "genre": "Electronic", "owner_id": 3, "title": "Pay Gated Download", "is_unlisted": "f",
		"download_conditions": []byte(`{"usdc_purchase": {"price": 135, "splits": [{"user_id": 3, "percentage": 100.0}]}}`), "is_downloadable": "t"},
	{"track_id": 302, "genre": "Electronic", "owner_id": 3, "title": "Tip Gated Stream", "is_unlisted": "f",
		"stream_conditions":   []byte(`{"tip_user_id": 3}`),
		"download_conditions": []byte(`{"tip_user_id": 3}`), "is_downloadable": "f"},
	{"track_id": 303, "genre": "Electronic", "owner_id": 3, "title": "Pay Gated Stream", "is_unlisted": "f",
		"stream_conditions": []byte(`{"usdc_purchase": {"price": 135, "splits": [{"user_id": 3, "percentage": 100.0}]}}`),
		"is_downloadable":   "f"},
	{"track_id": 400, "genre": "Folk", "owner_id": 5, "title": "Trending Month Folk", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 500, "genre": "Experimental", "owner_id": 6, "title": "track by permalink", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 501, "genre": "Folk", "owner_id": 301, "title": "Trending Popular user Track", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 502, "genre": "Folk", "owner_id": 302, "title": "Trending Folows Everyone Track", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 503, "genre": "Electronic", "owner_id": 1, "title": "Trending Electronic Track 1", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 504, "genre": "Disco", "owner_id": 2, "title": "Trending Disco Track 1", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 505, "genre": "Rock", "owner_id": 3, "title": "Trending Rock Track 1", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 506, "genre": "Pop", "owner_id": 4, "title": "Trending Gated Pop Track 1", "is_unlisted": "f",
		"stream_conditions": []byte(`{"usdc_purchase": {"price": 135, "splits": [{"user_id": 4, "percentage": 100.0}]}}`),
		"is_downloadable":   "f"},
	{"track_id": 507, "genre": "Jazz", "owner_id": 5, "title": "Trending Gated Jazz Track 1", "is_unlisted": "f",
		"stream_conditions": []byte(`{"usdc_purchase": {"price": 135, "splits": [{"user_id": 5, "percentage": 100.0}]}}`),
		"is_downloadable":   "f"},
	{"track_id": 508, "genre": "Classical", "owner_id": 6, "title": "Trending Gated Classical Track 1", "is_unlisted": "f",
		"stream_conditions": []byte(`{"usdc_purchase": {"price": 135, "splits": [{"user_id": 6, "percentage": 100.0}]}}`),
		"is_downloadable":   "f"},
	{"track_id": 509, "genre": "Electronic", "owner_id": 7, "title": "Trending Electronic Track 2", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 510, "genre": "Disco", "owner_id": 8, "title": "Trending Disco Track 2", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 511, "genre": "Rock", "owner_id": 11, "title": "Trending Rock Track 2", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 512, "genre": "Pop", "owner_id": 1, "title": "Trending Pop Track 2", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 513, "genre": "Jazz", "owner_id": 2, "title": "Trending Jazz Track 2", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 514, "genre": "Classical", "owner_id": 3, "title": "Trending Classical Track 2", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 515, "genre": "Electronic", "owner_id": 4, "title": "Trending Electronic Track 3", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 516, "genre": "Disco", "owner_id": 5, "title": "Trending Disco Track 3", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 517, "genre": "Rock", "owner_id": 6, "title": "Trending Rock Track 3", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 518, "genre": "Pop", "owner_id": 7, "title": "Trending Pop Track 3", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 519, "genre": "Jazz", "owner_id": 8, "title": "Trending Jazz Track 3", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 520, "genre": "Classical", "owner_id": 11, "title": "Trending Classical Track 3", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 521, "genre": "Electronic", "owner_id": 1, "title": "Trending Electronic Track 4", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 522, "genre": "Disco", "owner_id": 2, "title": "Trending Disco Track 4", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 523, "genre": "Rock", "owner_id": 3, "title": "Trending Rock Track 4", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 524, "genre": "Pop", "owner_id": 4, "title": "Trending Pop Track 4", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 525, "genre": "Jazz", "owner_id": 5, "title": "Trending Jazz Track 4", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 526, "genre": "Classical", "owner_id": 6, "title": "Trending Classical Track 4", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 527, "genre": "Electronic", "owner_id": 7, "title": "Trending Electronic Track 5", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 528, "genre": "Disco", "owner_id": 8, "title": "Trending Disco Track 5", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 529, "genre": "Rock", "owner_id": 11, "title": "Trending Rock Track 5", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 530, "genre": "Pop", "owner_id": 1, "title": "Trending Pop Track 5", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 531, "genre": "Jazz", "owner_id": 2, "title": "Trending Jazz Track 5", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 532, "genre": "Classical", "owner_id": 3, "title": "Trending Classical Track 5", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 533, "genre": "Electronic", "owner_id": 4, "title": "Trending Electronic Track 6", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 534, "genre": "Disco", "owner_id": 5, "title": "Trending Disco Track 6", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 535, "genre": "Rock", "owner_id": 6, "title": "Trending Rock Track 6", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 536, "genre": "Pop", "owner_id": 7, "title": "Trending Pop Track 6", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 537, "genre": "Jazz", "owner_id": 8, "title": "Trending Jazz Track 6", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 538, "genre": "Classical", "owner_id": 11, "title": "Trending Classical Track 6", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 539, "genre": "Electronic", "owner_id": 1, "title": "Trending Electronic Track 7", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 540, "genre": "Disco", "owner_id": 2, "title": "Trending Disco Track 7", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 541, "genre": "Rock", "owner_id": 3, "title": "Trending Rock Track 7", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 542, "genre": "Pop", "owner_id": 4, "title": "Trending Pop Track 7", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 543, "genre": "Jazz", "owner_id": 5, "title": "Trending Jazz Track 7", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 544, "genre": "Classical", "owner_id": 6, "title": "Trending Classical Track 7", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 545, "genre": "Electronic", "owner_id": 7, "title": "Trending Electronic Track 8", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 546, "genre": "Disco", "owner_id": 8, "title": "Trending Disco Track 8", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 547, "genre": "Rock", "owner_id": 11, "title": "Trending Rock Track 8", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 548, "genre": "Pop", "owner_id": 1, "title": "Trending Pop Track 8", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 549, "genre": "Jazz", "owner_id": 2, "title": "Trending Jazz Track 8", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 550, "genre": "Classical", "owner_id": 3, "title": "Trending Classical Track 8", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 551, "genre": "Electronic", "owner_id": 4, "title": "Trending Electronic Track 9", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 552, "genre": "Disco", "owner_id": 5, "title": "Trending Disco Track 9", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 553, "genre": "Rock", "owner_id": 6, "title": "Trending Rock Track 9", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 554, "genre": "Pop", "owner_id": 7, "title": "Trending Pop Track 9", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 555, "genre": "Jazz", "owner_id": 8, "title": "Trending Jazz Track 9", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 556, "genre": "Classical", "owner_id": 11, "title": "Trending Classical Track 9", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 557, "genre": "Electronic", "owner_id": 1, "title": "Trending Electronic Track 10", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 558, "genre": "Disco", "owner_id": 2, "title": "Trending Disco Track 10", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 559, "genre": "Rock", "owner_id": 3, "title": "Trending Rock Track 10", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 560, "genre": "Pop", "owner_id": 4, "title": "Trending Pop Track 10", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 561, "genre": "Jazz", "owner_id": 5, "title": "Trending Jazz Track 10", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 562, "genre": "Classical", "owner_id": 6, "title": "Trending Classical Track 10", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 563, "genre": "Electronic", "owner_id": 7, "title": "Trending Electronic Track 11", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 564, "genre": "Disco", "owner_id": 8, "title": "Trending Disco Track 11", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 565, "genre": "Rock", "owner_id": 11, "title": "Trending Rock Track 11", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 566, "genre": "Pop", "owner_id": 1, "title": "Trending Pop Track 11", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 567, "genre": "Jazz", "owner_id": 2, "title": "Trending Jazz Track 11", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 568, "genre": "Classical", "owner_id": 3, "title": "Trending Classical Track 11", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 569, "genre": "Electronic", "owner_id": 4, "title": "Trending Electronic Track 12", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 570, "genre": "Disco", "owner_id": 5, "title": "Trending Disco Track 12", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 571, "genre": "Rock", "owner_id": 6, "title": "Trending Rock Track 12", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 572, "genre": "Pop", "owner_id": 7, "title": "Trending Pop Track 12", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 573, "genre": "Jazz", "owner_id": 8, "title": "Trending Jazz Track 12", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 574, "genre": "Classical", "owner_id": 11, "title": "Trending Classical Track 12", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 575, "genre": "Electronic", "owner_id": 1, "title": "Trending Electronic Track 13", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 576, "genre": "Disco", "owner_id": 2, "title": "Trending Disco Track 13", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 577, "genre": "Rock", "owner_id": 3, "title": "Trending Rock Track 13", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 578, "genre": "Pop", "owner_id": 4, "title": "Trending Pop Track 13", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 579, "genre": "Jazz", "owner_id": 5, "title": "Trending Jazz Track 13", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 580, "genre": "Classical", "owner_id": 6, "title": "Trending Classical Track 13", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 581, "genre": "Electronic", "owner_id": 7, "title": "Trending Electronic Track 14", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 582, "genre": "Disco", "owner_id": 8, "title": "Trending Disco Track 14", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 583, "genre": "Rock", "owner_id": 11, "title": "Trending Rock Track 14", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 584, "genre": "Pop", "owner_id": 1, "title": "Trending Pop Track 14", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 585, "genre": "Jazz", "owner_id": 2, "title": "Trending Jazz Track 14", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 586, "genre": "Classical", "owner_id": 3, "title": "Trending Classical Track 14", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 587, "genre": "Electronic", "owner_id": 4, "title": "Trending Electronic Track 15", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 588, "genre": "Disco", "owner_id": 5, "title": "Trending Disco Track 15", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 589, "genre": "Rock", "owner_id": 6, "title": "Trending Rock Track 15", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 590, "genre": "Pop", "owner_id": 7, "title": "Trending Pop Track 15", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 591, "genre": "Jazz", "owner_id": 8, "title": "Trending Jazz Track 15", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 592, "genre": "Classical", "owner_id": 11, "title": "Trending Classical Track 15", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 593, "genre": "Electronic", "owner_id": 1, "title": "Trending Electronic Track 16", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 594, "genre": "Disco", "owner_id": 2, "title": "Trending Disco Track 16", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 595, "genre": "Rock", "owner_id": 3, "title": "Trending Rock Track 16", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 596, "genre": "Pop", "owner_id": 4, "title": "Trending Pop Track 16", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 597, "genre": "Jazz", "owner_id": 5, "title": "Underground Trending Jazz Track 16", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 598, "genre": "Classical", "owner_id": 6, "title": "Underground Trending Classical Track 16", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 599, "genre": "Electronic", "owner_id": 7, "title": "Underground Trending Electronic Track 17", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 600, "genre": "Disco", "owner_id": 8, "title": "Underground Trending Disco Track 17", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 601, "genre": "Rock", "owner_id": 11, "title": "Underground Trending Rock Track 17", "is_unlisted": "f", "is_downloadable": "f"},
	{"track_id": 602, "genre": "Pop", "owner_id": 1, "title": "Underground Trending Pop Track 17", "is_unlisted": "f", "is_downloadable": "f"},

	// data for v1_user_tracks_test.go
	{"track_id": 700, "genre": "Electronic", "owner_id": 500, "title": "UserTracksTester Track 1", "created_at": "2021-01-01 00:00:00"},
	{"track_id": 701, "genre": "Electronic", "owner_id": 500, "title": "UserTracksTester Track 4", "created_at": "2021-01-03 00:00:00"},
	{"track_id": 702, "genre": "Electronic", "owner_id": 500, "title": "UserTracksTester Track 3", "created_at": "2021-01-04 00:00:00"},
	// created before other track but later release date
	{"track_id": 703, "genre": "Electronic", "owner_id": 500, "title": "UserTracksTester Track 2", "release_date": "2021-01-05 00:00:00", "created_at": "2021-01-02 00:00:00"},
}

/*
track_id,genre,owner_id,title,is_unlisted,stream_conditions,download_conditions,is_downloadable
100,Electronic,1,T1,f,,,t
101,Alternative,1,T2,f,,,f
200,Electronic,2,Culca Canyon,f,,,f
201,Alternative,2,Turkey Time DEMO,t,,,f
202,Alternative,2,Turkey Time (live),f,,,f
300,Electronic,3,Follow Gated Download,f,,"{""follow_user_id"": 3}",t
301,Electronic,3,Pay Gated Download,f,,"{""usdc_purchase"": {""price"": 135, ""splits"": [{""user_id"": 3, ""percentage"": 100.0}]}}",t
302,Electronic,3,Tip Gated Stream,f,"{""tip_user_id"": 3}","{""tip_user_id"": 3}",f
303,Electronic,3,Pay Gated Stream,f,"{""usdc_purchase"": {""price"": 135, ""splits"": [{""user_id"": 3, ""percentage"": 100.0}]}}",,f
400,Folk,5,Trending Month Folk,f,,,f
500,Experimental,6,track by permalink,f,,,f
501,Folk,301,Trending Popular user Track,f,,,f
502,Folk,302,Trending Folows Everyone Track,f,,,f
503,Electronic,1,Trending Electronic Track 1,f,,,f
504,Disco,2,Trending Disco Track 1,f,,,f
505,Rock,3,Trending Rock Track 1,f,,,f
506,Pop,4,Trending Gated Pop Track 1,f,"{""usdc_purchase"": {""price"": 135, ""splits"": [{""user_id"": 4, ""percentage"": 100.0}]}}",,f
507,Jazz,5,Trending Gated Jazz Track 1,f,"{""usdc_purchase"": {""price"": 135, ""splits"": [{""user_id"": 5, ""percentage"": 100.0}]}}",,f
508,Classical,6,Trending Gated Classical Track 1,f,"{""usdc_purchase"": {""price"": 135, ""splits"": [{""user_id"": 6, ""percentage"": 100.0}]}}",,f
509,Electronic,7,Trending Electronic Track 2,f,,,f
510,Disco,8,Trending Disco Track 2,f,,,f
511,Rock,11,Trending Rock Track 2,f,,,f
512,Pop,1,Trending Pop Track 2,f,,,f
513,Jazz,2,Trending Jazz Track 2,f,,,f
514,Classical,3,Trending Classical Track 2,f,,,f
515,Electronic,4,Trending Electronic Track 3,f,,,f
516,Disco,5,Trending Disco Track 3,f,,,f
517,Rock,6,Trending Rock Track 3,f,,,f
518,Pop,7,Trending Pop Track 3,f,,,f
519,Jazz,8,Trending Jazz Track 3,f,,,f
520,Classical,11,Trending Classical Track 3,f,,,f
521,Electronic,1,Trending Electronic Track 4,f,,,f
522,Disco,2,Trending Disco Track 4,f,,,f
523,Rock,3,Trending Rock Track 4,f,,,f
524,Pop,4,Trending Pop Track 4,f,,,f
525,Jazz,5,Trending Jazz Track 4,f,,,f
526,Classical,6,Trending Classical Track 4,f,,,f
527,Electronic,7,Trending Electronic Track 5,f,,,f
528,Disco,8,Trending Disco Track 5,f,,,f
529,Rock,11,Trending Rock Track 5,f,,,f
530,Pop,1,Trending Pop Track 5,f,,,f
531,Jazz,2,Trending Jazz Track 5,f,,,f
532,Classical,3,Trending Classical Track 5,f,,,f
533,Electronic,4,Trending Electronic Track 6,f,,,f
534,Disco,5,Trending Disco Track 6,f,,,f
535,Rock,6,Trending Rock Track 6,f,,,f
536,Pop,7,Trending Pop Track 6,f,,,f
537,Jazz,8,Trending Jazz Track 6,f,,,f
538,Classical,11,Trending Classical Track 6,f,,,f
539,Electronic,1,Trending Electronic Track 7,f,,,f
540,Disco,2,Trending Disco Track 7,f,,,f
541,Rock,3,Trending Rock Track 7,f,,,f
542,Pop,4,Trending Pop Track 7,f,,,f
543,Jazz,5,Trending Jazz Track 7,f,,,f
544,Classical,6,Trending Classical Track 7,f,,,f
545,Electronic,7,Trending Electronic Track 8,f,,,f
546,Disco,8,Trending Disco Track 8,f,,,f
547,Rock,11,Trending Rock Track 8,f,,,f
548,Pop,1,Trending Pop Track 8,f,,,f
549,Jazz,2,Trending Jazz Track 8,f,,,f
550,Classical,3,Trending Classical Track 8,f,,,f
551,Electronic,4,Trending Electronic Track 9,f,,,f
552,Disco,5,Trending Disco Track 9,f,,,f
553,Rock,6,Trending Rock Track 9,f,,,f
554,Pop,7,Trending Pop Track 9,f,,,f
555,Jazz,8,Trending Jazz Track 9,f,,,f
556,Classical,11,Trending Classical Track 9,f,,,f
557,Electronic,1,Trending Electronic Track 10,f,,,f
558,Disco,2,Trending Disco Track 10,f,,,f
559,Rock,3,Trending Rock Track 10,f,,,f
560,Pop,4,Trending Pop Track 10,f,,,f
561,Jazz,5,Trending Jazz Track 10,f,,,f
562,Classical,6,Trending Classical Track 10,f,,,f
563,Electronic,7,Trending Electronic Track 11,f,,,f
564,Disco,8,Trending Disco Track 11,f,,,f
565,Rock,11,Trending Rock Track 11,f,,,f
566,Pop,1,Trending Pop Track 11,f,,,f
567,Jazz,2,Trending Jazz Track 11,f,,,f
568,Classical,3,Trending Classical Track 11,f,,,f
569,Electronic,4,Trending Electronic Track 12,f,,,f
570,Disco,5,Trending Disco Track 12,f,,,f
571,Rock,6,Trending Rock Track 12,f,,,f
572,Pop,7,Trending Pop Track 12,f,,,f
573,Jazz,8,Trending Jazz Track 12,f,,,f
574,Classical,11,Trending Classical Track 12,f,,,f
575,Electronic,1,Trending Electronic Track 13,f,,,f
576,Disco,2,Trending Disco Track 13,f,,,f
577,Rock,3,Trending Rock Track 13,f,,,f
578,Pop,4,Trending Pop Track 13,f,,,f
579,Jazz,5,Trending Jazz Track 13,f,,,f
580,Classical,6,Trending Classical Track 13,f,,,f
581,Electronic,7,Trending Electronic Track 14,f,,,f
582,Disco,8,Trending Disco Track 14,f,,,f
583,Rock,11,Trending Rock Track 14,f,,,f
584,Pop,1,Trending Pop Track 14,f,,,f
585,Jazz,2,Trending Jazz Track 14,f,,,f
586,Classical,3,Trending Classical Track 14,f,,,f
587,Electronic,4,Trending Electronic Track 15,f,,,f
588,Disco,5,Trending Disco Track 15,f,,,f
589,Rock,6,Trending Rock Track 15,f,,,f
590,Pop,7,Trending Pop Track 15,f,,,f
591,Jazz,8,Trending Jazz Track 15,f,,,f
592,Classical,11,Trending Classical Track 15,f,,,f
593,Electronic,1,Trending Electronic Track 16,f,,,f
594,Disco,2,Trending Disco Track 16,f,,,f
595,Rock,3,Trending Rock Track 16,f,,,f
596,Pop,4,Trending Pop Track 16,f,,,f
597,Jazz,5,Underground Trending Jazz Track 16,f,,,f
598,Classical,6,Underground Trending Classical Track 16,f,,,f
599,Electronic,7,Underground Trending Electronic Track 17,f,,,f
600,Disco,8,Underground Trending Disco Track 17,f,,,f
601,Rock,11,Underground Trending Rock Track 17,f,,,f
602,Pop,1,Underground Trending Pop Track 17,f,,,f
*/
