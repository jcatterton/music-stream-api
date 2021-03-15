// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	http "net/http"

	youtube "github.com/kkdai/youtube/v2"
	mock "github.com/stretchr/testify/mock"
)

// YoutubeClient is an autogenerated mock type for the YoutubeClient type
type YoutubeClient struct {
	mock.Mock
}

// GetStream provides a mock function with given fields: video, format
func (_m *YoutubeClient) GetStream(video *youtube.Video, format *youtube.Format) (*http.Response, error) {
	ret := _m.Called(video, format)

	var r0 *http.Response
	if rf, ok := ret.Get(0).(func(*youtube.Video, *youtube.Format) *http.Response); ok {
		r0 = rf(video, format)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*http.Response)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*youtube.Video, *youtube.Format) error); ok {
		r1 = rf(video, format)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVideo provides a mock function with given fields: videoId
func (_m *YoutubeClient) GetVideo(videoId string) (*youtube.Video, error) {
	ret := _m.Called(videoId)

	var r0 *youtube.Video
	if rf, ok := ret.Get(0).(func(string) *youtube.Video); ok {
		r0 = rf(videoId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*youtube.Video)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(videoId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
